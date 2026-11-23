# Cross-language benchmark suite (#190)

The same six programs in Zhex (interpreter AND native), Go, C, JavaScript
(Node and Bun), Ruby, Python, and PHP, with one harness that builds,
verifies, and times them. This extends the #184/#6 performance story: how
does Zhex stack against the languages people actually compare?

## Fairness law

- **Same algorithm, same data-structure shapes** per benchmark in every
  language — no language-specific shortcuts, no benchmark-game tricks.
- **Each engine's ordinary idiomatic form**: Python joins a list, Go uses
  `strings.Builder`, Zhex uses a `Buffer` or `List` — the natural instrument
  each language ships for the job, with no micro-tuning beyond that.
- **Warm-up + median of five** measured runs per engine/benchmark.
- **Outputs must be equal**: before any timing, the harness runs every
  engine on every benchmark and byte-compares stdout; any mismatch is a
  hard failure with the diff. The suite's gate is this verification.
- **Versions pinned in the output** (`performance/results.txt` records each
  engine's version, host, and date).
- **Absent engines are marked honestly** — the harness discovers runtimes at
  run time; a missing runtime reads `NOT INSTALLED`, never an approximation.
- **Peak RSS, when `/usr/bin/time` is installed**, is the median of five more,
  untimed runs after the timed ones. The timed runs always execute the bare
  command: GNU time adds about 0.7 ms to a run, which would land in every
  wall number. Honest caveat: the number **includes each engine's own
  runtime baseline** — Node/Ruby/Python carry tens of MB of runtime, C is
  tiny, Go binaries a few MB, zhex-interp includes the Go runtime plus the
  interpreter. The column compares engines-as-deployed, not algorithms in
  isolation, and says so. When GNU `/usr/bin/time` is absent, the column is
  marked unavailable rather than approximated.

## Benchmarks

One directory per benchmark under `benchmarks/`, one file per engine — the
same algorithm and identical observable work everywhere. `wordcount` and
`filterstream` read fixtures from `inputs/` (regenerated deterministically
by the harness from `inputs/gen.py`, seed 190); `filterstream` reads the
fixture on stdin.

| name | shape | work |
|---|---|---|
| `fibrec` | recursion | naive recursive fib(30), print it |
| `intloop` | integer loops | gcd by remainder over 100k pairs from an in-program Park-Miller generator, print the sum |
| `stringbuild` | strings | 200k numbered lines into one text, print its length and how often `"42"` occurs (per line, non-overlapping) |
| `wordcount` | maps/collections | word frequencies over a generated ~2 MB corpus, print the top ten (count desc, word asc) |
| `gridsim` | 2-D arrays | double-buffer heat diffusion on a 400x400 grid, 10 generations, print the final cell sum |
| `filterstream` | streaming I/O | print stdin lines containing `ERROR` (60k-line fixture) |

Determinism notes: `intloop`'s generator is in-program (multiplier 48271,
modulus 2147483647, seed 20260119 — every intermediate stays under 2^53 so
JavaScript's doubles are exact); `wordcount` keeps a first-seen distinct-word
list so no language's map iteration order can leak into the output;
`gridsim` uses only positive truncating division, identical in every engine.
The Zhex programs follow the repo language law and use `word main`, so the
same file runs under `zhex run` and `zhex compile --target=native`.

## How to run

```sh
just perf          # or: performance/run.sh
```

The harness discovers engines, rebuilds `bin/zhex` and the native binaries
(`zhex compile --target=native`), regenerates the inputs, verifies outputs,
then prints the table to stdout and writes `performance/results.txt`
(gitignored artifacts live in `performance/build/` and `inputs/*.txt`).
Expect a few minutes: the interpreter's gridsim row dominates.

## Results

Recorded 2026-11-23 from one `just perf` run on a calm machine (the raw
copy is `results.txt`); median wall time in ms, 1 warm-up + 5 measured
runs, and median peak RSS in KB from five more untimed runs. Machine: AMD
Ryzen 7 6800H, Linux 7.2.15-1-omarchy x86_64, with the engines exactly as
the report records them (php is not installed, so its column reads NOT
INSTALLED):

| engine | version |
|---|---|
| zhex-interp | zhex b4657c1 go version go1.27.4 linux/amd64 |
| zhex-native | zhex b4657c1 go version go1.27.4 linux/amd64 |
| go | go version go1.27.4 linux/amd64 |
| c | cc (GCC) 16.2.1 20260810 |
| node | v22.23.1 |
| bun | bun 1.4.4 |
| ruby | ruby 3.4.10 |
| python3 | Python 3.14.8 |
| php | NOT INSTALLED |

### Median wall time (ms)

| benchmark | zhex-interp|zhex-native|go|c|node|bun|ruby|python3|php |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:||
| fibrec | 1354.6 | 24.9 | 8.7 | 3.7 | 43.5 | 36.7 | 105.2 | 102.7 | NOT INSTALLED |
| intloop | 873.0 | 265.9 | 7.9 | 7.1 | 46.2 | 52.9 | 75.7 | 87.9 | NOT INSTALLED |
| stringbuild | 389.9 | 34.4 | 33.6 | 14.8 | 98.2 | 81.6 | 172.7 | 84.8 | NOT INSTALLED |
| wordcount | 248.8 | 15.2 | 11.9 | 10.6 | 51.9 | 57.0 | 106.6 | 50.1 | NOT INSTALLED |
| gridsim | 4323.6 | 394.3 | 8.1 | 5.1 | 59.9 | 51.9 | 337.2 | 714.0 | NOT INSTALLED |
| filterstream | 137.8 | 13.1 | 7.3 | 4.9 | 62.1 | 63.7 | 71.0 | 18.8 | NOT INSTALLED |
| *geomean (ms)* | 639.4 | 51.1 | 10.8 | 6.8 | 58.0 | 55.7 | 123.3 | 89.5 | — |

### Median peak RSS (KB)

| benchmark | zhex-interp|zhex-native|go|c|node|bun|ruby|python3|php |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:||
| fibrec | 16568 | 6044 | 3568 | 1752 | 54040 | 44212 | 13396 | 9384 | NOT INSTALLED |
| intloop | 16388 | 10920 | 5624 | 1640 | 56060 | 44332 | 13376 | 9480 | NOT INSTALLED |
| stringbuild | 61060 | 41752 | 34476 | 13812 | 103004 | 69516 | 62920 | 42628 | NOT INSTALLED |
| wordcount | 23792 | 12816 | 12184 | 4376 | 61492 | 44284 | 18700 | 14796 | NOT INSTALLED |
| gridsim | 116832 | 17292 | 7784 | 4344 | 62032 | 44404 | 15956 | 12180 | NOT INSTALLED |
| filterstream | 14836 | 10228 | 10012 | 1560 | 61764 | 45280 | 14300 | 9736 | NOT INSTALLED |

Each memory number is the engine's whole process at its largest — runtime
included — so the column compares engines as shipped. The interpreter's
gridsim row is the outlier, about 117 MB against 17 MB for the native
build, because the interpreter's collections are pointer-based. Node and
Bun carry 44-62 MB of runtime, and C sits at 1.6-14 MB.

Reading it honestly:

- **zhex-native lands with the JIT engines**: geomean 51.1 ms vs node
  58.0 and bun 55.7 — faster than both on four of six benchmarks. The
  clear exceptions are `intloop` (266 ms) and `gridsim`: the native
  target keeps the language's 128-bit `Int` contract, so hot integer
  division pays big-Int costs the 64-bit engines do not.
- **zhex-interp is 3.3-54x the native column** per workload (geomean
  12.5x), consistent with the #6 three-engine table — the same fib-rec
  1355 ms / 25 ms pair reproduces here.
- **Compiled vs scripted is one order of magnitude** on this mix: go
  10.8 / c 6.8 vs node 58.0 / bun 55.7 / python 89.5 / ruby 123.3.
- Surprises worth knowing: Python 3.14 beats Ruby on four of six here
  (Ruby's loops win `intloop` and `gridsim`) and beats the JS engines on
  the map and I-O shapes; Node beats Bun on short-lived scripts (startup,
  not steady-state); Python's C-level line loop makes `filterstream` the
  fastest scripted run at 18.8 ms.

Run-to-run movement: a three-run smoke check two minutes earlier put C's
fibrec row at 3.2 ms and Go's at 6.8 ms, where this run shows 3.7 and
8.7 — rows of a few milliseconds move by up to a third between runs, so
compare rows and orders of magnitude, not single cells. The memory
column was measured twice that day, once under heavy load and once on a
calm machine, and the two agreed within one percent.

## Honest caveats

- The table above is from **one real run on one machine** — an indication
  for that environment, not an authoritative language ranking. Run `just
  perf` on your own machine to get your numbers.
- Times are wall-clock per process, so interpreted engines carry their
  startup cost — that is engines-as-deployed, which is the honest comparison
  for scripts.
- Zhex `Int` is 128-bit (the interpreter's contract); the other engines use
  native 64-bit integers. Benchmark values stay far below 64 bits, so
  outputs are identical, but the Zhex engines do wider arithmetic to do it.
- The Zhex interpreter binary embeds the checker; both Zhex engines pay for
  128-bit integers and checked semantics by design.
- `zhex-native` numbers include a Go-runtime baseline (GC, scheduler) the
  C column does not have; the Go column carries the same baseline, which is
  the apples-to-apples comparison for it.
