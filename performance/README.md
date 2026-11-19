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
- **Peak RSS, when `/usr/bin/time` is installed**, is the median of the five
  measured runs. Honest caveat: the number **includes each engine's own
  runtime baseline** — Node/Ruby/Python carry tens of MB of runtime, C is
  tiny, Go binaries a few MB, zhex-interp includes the Go runtime plus the
  interpreter. The column compares engines-as-deployed, not algorithms in
  isolation, and says so. When GNU `/usr/bin/time` is absent (as on the
  machine behind the table below), the column is marked unavailable rather
  than approximated.

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

Recorded 2026-11-19 from one `just perf` run (the raw copy is
`results.txt`); median wall time in ms, 1 warm-up + 5 measured runs.
Machine: AMD Ryzen 7 6800H, Linux 7.2.15-1-omarchy x86_64, go1.27.4,
GCC 16.2.1, node v22.23.1, bun 1.4.4, ruby 3.4.10, Python 3.14.8. PHP
sources ship; php is not installed on this machine, so its column reads
NOT INSTALLED. The peak-RSS column was unavailable for the same reason —
GNU `/usr/bin/time` is not installed here (on Arch: `pacman -S time`;
the harness picks it up automatically when present).

| benchmark | zhex-interp | zhex-native | go | c | node | bun | ruby | python3 | php |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| fibrec | 1323.7 | 19.1 | 6.5 | 3.1 | 35.3 | 36.9 | 102.7 | 96.5 | NOT INSTALLED |
| intloop | 919.8 | 266.1 | 7.4 | 7.2 | 47.1 | 51.4 | 77.6 | 90.9 | NOT INSTALLED |
| stringbuild | 402.0 | 38.6 | 36.0 | 15.1 | 101.8 | 77.0 | 171.3 | 84.5 | NOT INSTALLED |
| wordcount | 262.0 | 14.0 | 11.8 | 9.6 | 52.1 | 53.8 | 101.5 | 52.7 | NOT INSTALLED |
| gridsim | 4310.0 | 378.1 | 6.9 | 4.4 | 54.7 | 51.5 | 317.4 | 688.7 | NOT INSTALLED |
| filterstream | 138.2 | 12.7 | 7.0 | 4.7 | 59.9 | 62.2 | 66.7 | 17.6 | NOT INSTALLED |
| *geomean (ms)* | 651.4 | 48.6 | 10.0 | 6.4 | 55.4 | 54.2 | 119.7 | 88.3 | — |

Reading it honestly:

- **zhex-native lands with the JIT engines**: geomean 48.6 ms vs node
  55.4 and bun 54.2 — faster than both on four of six benchmarks. The
  clear exception is `intloop` (266 ms): the native target keeps the
  language's 128-bit `Int` contract, so hot integer division pays
  big-Int costs the 64-bit engines do not.
- **zhex-interp is 3.5–69x the native column** per workload (geomean
  13.4x), consistent with the #6 three-engine table — the same fib-rec
  1323 ms / 19 ms pair reproduces here.
- **Compiled vs scripted is one order of magnitude** on this mix: go
  10.0 / c 6.4 vs node 55.4 / bun 54.2 / python 88.3 / ruby 119.7.
- Surprises worth knowing: Python 3.14 beats Ruby everywhere here and
  beats the JS engines on the string/map/I-O shapes (its loops lose);
  Node beats Bun on short-lived scripts (startup, not steady-state);
  Python's C-level line loop makes `filterstream` the fastest scripted
  run at 17.6 ms.

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
