#!/usr/bin/env bash
# Cross-language benchmark harness (#190).
#
# Discovers engines at run time, builds the compiled artifacts, regenerates
# the deterministic inputs, verifies every engine's output is byte-identical
# BEFORE any timing (a mismatch is a hard failure with the diff), then runs
# one warm-up plus five measured runs per engine/benchmark and reports the
# median wall time in ms (and median peak RSS in KB when /usr/bin/time is
# installed). Absent engines are marked honestly, never approximated.
#
# Fairness law (see README.md): same algorithm and data-structure shapes per
# benchmark in every language, each engine's ordinary idiomatic form, no
# micro-tuning, warm-up + median of five, versions pinned in the output,
# results are environment-specific indications.

set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$HERE")"
BIN="$ROOT/bin/zhex"
BUILD="$HERE/build"
INPUTS="$HERE/inputs"
RESULTS="$HERE/results.txt"
BENCHES="fibrec intloop stringbuild wordcount gridsim filterstream"
ENGINES="zhex-interp zhex-native go c node bun ruby python3 php"
RUNS=5

log() { printf '%s\n' "$*" >&2; }

mkdir -p "$BUILD"

# ---------------------------------------------------------------- discovery

declare -A ABSENT=() VERSION=() DEPS
DEPS[zhex-interp]=""
DEPS[zhex-native]="go"
DEPS[go]="go"
DEPS[c]="cc"
DEPS[node]="node"
DEPS[bun]="bun"
DEPS[ruby]="ruby"
DEPS[python3]="python3"
DEPS[php]="php"

GNU_TIME=""
if [ -x /usr/bin/time ] && /usr/bin/time --version 2>&1 | grep -q GNU; then
    GNU_TIME=/usr/bin/time
fi

command?() { command -v "$1" >/dev/null 2>&1; }

# The interpreter binary: rebuild when Go is present, else reuse bin/zhex.
if command? go; then
    log "building bin/zhex"
    (cd "$ROOT" && go build -o bin/zhex ./cmd/zhex)
fi
if [ ! -x "$BIN" ]; then
    log "bin/zhex missing and Go absent — marking both Zhex engines absent"
    ABSENT[zhex-interp]=1
    ABSENT[zhex-native]=1
fi

for engine in $ENGINES; do
    [ -n "${ABSENT[$engine]:-}" ] && continue
    missing=""
    for dep in ${DEPS[$engine]}; do
        command? "$dep" || missing="$dep"
    done
    if [ -n "$missing" ]; then
        ABSENT[$engine]=1
        log "engine $engine absent: $missing not installed"
    fi
done

version_of() {
    case $1 in
    zhex-interp | zhex-native)
        echo "zhex $(git -C "$ROOT" rev-parse --short HEAD) $(go version 2>/dev/null || echo '(prebuilt)')"
        ;;
    go) go version ;;
    c) cc --version | head -1 ;;
    node) node --version ;;
    bun) echo "bun $(bun --version)" ;;
    ruby) ruby --version | cut -d' ' -f1-2 ;;
    python3) python3 --version 2>&1 ;;
    php) php --version 2>/dev/null | head -1 ;;
    esac
}

# --------------------------------------------------------------------- build

for bench in $BENCHES; do
    if [ -z "${ABSENT[zhex-native]:-}" ]; then
        log "native build: $bench"
        "$BIN" compile "$HERE/benchmarks/$bench/$bench.zhex" --target=native -o "$BUILD/$bench-native" >&2
    fi
    if [ -z "${ABSENT[go]:-}" ]; then
        log "go build: $bench"
        go build -o "$BUILD/$bench-go" "$HERE/benchmarks/$bench/$bench.go"
    fi
    if [ -z "${ABSENT[c]:-}" ]; then
        log "c build: $bench"
        cc -O2 -o "$BUILD/$bench-c" "$HERE/benchmarks/$bench/$bench.c"
    fi
done

if ! command? python3; then
    log "python3 is required to generate the benchmark inputs"
    exit 1
fi
log "generating inputs"
(cd "$HERE" && python3 inputs/gen.py) >&2

# ------------------------------------------------------------------ commands

engine_cmd() { # sets the global CMD array for (engine, benchmark)
    local engine=$1 bench=$2
    case $engine in
    zhex-interp) CMD=("$BIN" run "$HERE/benchmarks/$bench/$bench.zhex") ;;
    zhex-native) CMD=("$BUILD/$bench-native/$bench-native") ;;
    go) CMD=("$BUILD/$bench-go") ;;
    c) CMD=("$BUILD/$bench-c") ;;
    node) CMD=("node" "$HERE/benchmarks/$bench/$bench.js") ;;
    bun) CMD=("bun" "$HERE/benchmarks/$bench/$bench.js") ;;
    ruby) CMD=("ruby" "$HERE/benchmarks/$bench/$bench.rb") ;;
    python3) CMD=("python3" "$HERE/benchmarks/$bench/$bench.py") ;;
    php) CMD=("php" "$HERE/benchmarks/$bench/$bench.php") ;;
    *) return 1 ;;
    esac
}

execute() { # run one engine/benchmark from the suite dir (inputs are cwd-relative), stdout to $3
    local engine=$1 bench=$2 dest=$3
    engine_cmd "$engine" "$bench"
    if [ "$bench" = filterstream ]; then
        (cd "$HERE" && "${CMD[@]}") <"$INPUTS/filterstream.txt" >"$dest"
    else
        (cd "$HERE" && "${CMD[@]}") >"$dest"
    fi
}

# ------------------------------------------------------------------ verify

log "verifying outputs are identical across engines"
for bench in $BENCHES; do
    ref_engine=""
    for engine in $ENGINES; do
        [ -n "${ABSENT[$engine]:-}" ] && continue
        execute "$engine" "$bench" "$BUILD/out-$bench-$engine"
        if [ -z "$ref_engine" ]; then
            ref_engine=$engine
            continue
        fi
        if ! cmp -s "$BUILD/out-$bench-$ref_engine" "$BUILD/out-$bench-$engine"; then
            log "FAIL: $bench output mismatch: $engine differs from $ref_engine"
            diff "$BUILD/out-$bench-$ref_engine" "$BUILD/out-$bench-$engine" | head -20 >&2 || true
            exit 1
        fi
    done
    log "  $bench: $ref_engine + $(grep -c '' "$BUILD/out-$bench-$ref_engine") lines OK"
done

# --------------------------------------------------------------------- time

WALL_NS=0
RSS_KB=""

timed_execute() { # one measured run; sets WALL_NS and RSS_KB
    local engine=$1 bench=$2 start end
    engine_cmd "$engine" "$bench"
    start=$(date +%s%N)
    if [ -n "$GNU_TIME" ]; then
        if [ "$bench" = filterstream ]; then
            (cd "$HERE" && "$GNU_TIME" -f %M -o "$BUILD/rss.tmp" "${CMD[@]}") <"$INPUTS/filterstream.txt" >/dev/null
        else
            (cd "$HERE" && "$GNU_TIME" -f %M -o "$BUILD/rss.tmp" "${CMD[@]}") >/dev/null
        fi
        RSS_KB=$(cat "$BUILD/rss.tmp")
    else
        if [ "$bench" = filterstream ]; then
            (cd "$HERE" && "${CMD[@]}") <"$INPUTS/filterstream.txt" >/dev/null
        else
            (cd "$HERE" && "${CMD[@]}") >/dev/null
        fi
        RSS_KB=""
    fi
    end=$(date +%s%N)
    WALL_NS=$((end - start))
}

median() { printf '%s\n' "$@" | sort -n | awk -v n="$RUNS" 'NR == int((n + 1) / 2) {print; exit}'; }

for bench in $BENCHES; do
    for engine in $ENGINES; do
        [ -n "${ABSENT[$engine]:-}" ] && continue
        log "timing $bench / $engine"
        walls=()
        rsses=()
        execute "$engine" "$bench" /dev/null # warm-up
        for _ in $(seq $RUNS); do
            timed_execute "$engine" "$bench"
            walls+=("$WALL_NS")
            [ -n "$RSS_KB" ] && rsses+=("$RSS_KB")
        done
        median "${walls[@]}" >"$BUILD/time-$bench-$engine"
        if [ "${#rsses[@]}" -gt 0 ]; then
            median "${rsses[@]}" >"$BUILD/rss-$bench-$engine"
        else
            : >"$BUILD/rss-$bench-$engine"
        fi
    done
done

# -------------------------------------------------------------------- table

fmt_ms() { awk -v ns="$1" 'BEGIN {printf "%.1f", ns / 1000000}'; }

REPORT="$BUILD/report.md"
{
    echo "# Cross-language benchmark run"
    echo
    echo "- date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "- host: $(uname -srm)"
    echo "- runs: 1 warm-up + $RUNS measured, median wall time in ms"
    echo "- verify: all present engines produced byte-identical outputs before timing"
    if [ -n "$GNU_TIME" ]; then
        echo "- memory: median peak RSS in KB via /usr/bin/time %M (engines-as-deployed, runtime baseline included)"
    else
        echo "- memory: peak RSS column unavailable — GNU /usr/bin/time is not installed"
    fi
    echo
    echo "## Engines"
    echo
    echo "| engine | version |"
    echo "|---|---|"
    for engine in $ENGINES; do
        if [ -n "${ABSENT[$engine]:-}" ]; then
            echo "| $engine | NOT INSTALLED |"
        else
            echo "| $engine | $(version_of "$engine") |"
        fi
    done
    echo
    echo "## Median wall time (ms)"
    echo
    printf '| benchmark | %s |\n' "$(echo "$ENGINES" | tr ' ' '|')"
    printf '|---|%s|\n' "$(printf '%.0s---:|' $ENGINES)"
    for bench in $BENCHES; do
        row="| $bench "
        for engine in $ENGINES; do
            if [ -n "${ABSENT[$engine]:-}" ]; then
                row+="| NOT INSTALLED "
            else
                row+="| $(fmt_ms "$(cat "$BUILD/time-$bench-$engine")") "
            fi
        done
        echo "$row|"
    done
    geomean="| *geomean (ms)* "
    for engine in $ENGINES; do
        if [ -n "${ABSENT[$engine]:-}" ]; then
            geomean+="| — "
        else
            g=$(for bench in $BENCHES; do cat "$BUILD/time-$bench-$engine"; done |
                awk '{s += log($1 / 1000000); n++} END {if (n) printf "%.1f", exp(s / n)}')
            geomean+="| $g "
        fi
    done
    echo "$geomean|"
    echo
    if [ -n "$GNU_TIME" ]; then
        echo "## Median peak RSS (KB)"
        echo
        printf '| benchmark | %s |\n' "$(echo "$ENGINES" | tr ' ' '|')"
        printf '|---|%s|\n' "$(printf '%.0s---:|' $ENGINES)"
        for bench in $BENCHES; do
            row="| $bench "
            for engine in $ENGINES; do
                if [ -n "${ABSENT[$engine]:-}" ]; then
                    row+="| NOT INSTALLED "
                else
                    row+="| $(cat "$BUILD/rss-$bench-$engine") "
                fi
            done
            echo "$row|"
        done
    fi
} >"$REPORT"

cat "$REPORT"
cp "$REPORT" "$RESULTS"
log "wrote $RESULTS"
