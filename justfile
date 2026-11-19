# List available development commands.
default:
    @just --list

# Run interpreter and website tests.
test: doctests aoc
    go test ./...
    just build-site

# Check every example's ## > doctests and run its .test.zhex files with one built binary.
doctests: build
    bin/zhex test examples

# Run every AoC year's suite: all aoc/<year> *.test.zhex files with one built binary.
aoc: build
    bin/zhex test aoc

# Run the cross-language benchmark suite (#190): slow, not part of just test.
perf:
    ./performance/run.sh

# Run the WASM corpus gate: every examples/ file classified, in-subset byte-identical, out-of-subset fenced.
corpus:
    go test ./internal/wasm/ -run 'TestCorpusGate' -v

# Run tests with Go's race detector.
test-race:
    go test -race ./...

# Build the browser engine (#91): the interpreter as a wasm module under site/public/, named apart from the examples pass in site/public/wasm/. Skipped when its inputs are unchanged; just engine-fresh forces a rebuild.
engine:
    #!/bin/bash
    mkdir -p site/public
    hash="$(go version; find cmd/zhex/engine internal/lang -name '*.go' ! -name '*_test.go' -print0 | sort -z | xargs -0 sha256sum | sha256sum; sha256sum "$([ -f "$(go env GOROOT)/lib/wasm/wasm_exec.js" ] && echo "$(go env GOROOT)/lib/wasm/wasm_exec.js" || echo "$(go env GOROOT)/misc/wasm/wasm_exec.js")" cmd/zhex/engine/loader.js)"
    if [ -s site/public/zhex-engine.wasm ] && [ -f site/public/zhex-engine-exec.js ] && [ -f site/public/zhex-engine.js ] && [ -f site/public/.engine-hash ] && [ "$(cat site/public/.engine-hash)" = "$hash" ]; then
        echo "engine: up to date"
    else
        GOOS=js GOARCH=wasm go build -o site/public/zhex-engine.wasm ./cmd/zhex/engine &&
        { cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" site/public/zhex-engine-exec.js 2>/dev/null || cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" site/public/zhex-engine-exec.js; } &&
        cp cmd/zhex/engine/loader.js site/public/zhex-engine.js &&
        printf '%s' "$hash" > site/public/.engine-hash
    fi

# Force an engine rebuild, ignoring the hash cache.
engine-fresh:
    rm -f site/public/.engine-hash
    just engine

# Check Go source with vet, both builds.
check:
    go vet ./...
    GOOS=js GOARCH=wasm go vet ./cmd/zhex/engine ./internal/lang

# Benchmark the interpreter with allocation counts.
bench:
    go test -bench=. -benchmem -benchtime=2s ./internal/lang/

# Build the interpreter in bin/zhex.
build:
    go build -o bin/zhex ./cmd/zhex

# Build bin/zhex and copy it to a bin folder, default ~/.local/bin: just install [dir]
install $dir='$HOME/.local/bin': build
    mkdir -p "{{dir}}"
    cp bin/zhex "{{dir}}"

# Run an example by name, such as: just run fibonacci.
run $example='hello':
    go run ./cmd/zhex run "examples/$example.zhex"

# Build the website without serving it (what just test uses).
build-site: engine
    npm --prefix site run build

# Build the website from the working examples, the roadmap, and the browser engine, then serve it at http://127.0.0.1:8000/zhex/: just site [port]
site $port='8000': build-site
    npm --prefix site run preview -- --host 127.0.0.1 --port "{{port}}"

# Build the website with every wasm artifact recompiled, ignoring the hash caches.
site-fresh:
    ZHEX_WASM_FRESH=1 just engine-fresh
    ZHEX_WASM_FRESH=1 just build-site

# Prepare the local Zed language extension.
zed-prepare:
    go run ./tools/prepare-zed

# Install syntax support for Vim and Neovim.
vim-install:
    sh editors/vim/install.sh
