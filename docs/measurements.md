# Measurements: execution time and allocations

The text utility `text-stats` ([examples/text-stats.zhex](../examples/text-stats.zhex)) is the first real Zhex application and the workload for the first measurements. This page records how the shipped Go interpreter runs it, what it allocates, what the source says about readability, and a recommendation for the next application. It closes issue #4; the runs below are lang-smith's, dated 2026-11-13.

## Method

The benchmarks live in [bench_test.go](../internal/lang/bench_test.go), next to the interpreter, and run the real example programs:

```sh
go test -bench=. -benchmem -benchtime=2s ./internal/lang/
```

- `BenchmarkTextStatsCounts` — counts mode, one pass over the whole file (`file.read-all` plus the text counting words).
- `BenchmarkTextStatsUpper` — streaming mode, one `file.lines` loop printing each line uppercased.
- `BenchmarkCompileTextStats` — checking alone: parse and check the same source without running it.
- `BenchmarkFibonacci` — an interpreter baseline: recursive `fibonacci` at index 20.

A guard test, `TestBenchmarksRunRealPrograms`, runs each benchmarked program and asserts its output (prefix `20000 lines`, 20,000 uppercased lines, a compiled program, `6765`), so the benchmarks cannot silently degrade into timing no-ops.

The fixture is generated into a temporary directory: 20,000 lines (~0.7 MB) rotating through ten varied templates — café and naïve accents, a Japanese line, an em dash, a decimal comma and a euro sign, tabs and double spaces, a pangram, and one blank line. `just bench` reproduces the measurements with the same benchmarks, allocation reporting, and 2 s benchtime used for the tables below.

## Machine

go1.27.3, linux/amd64. AMD Ryzen 7 6800H with Radeon Graphics, x86_64, 16 logical CPUs; 8,866,780 kB (~8.5 GB) of memory available at run time.

## Results

First run:

```text
goos: linux
goarch: amd64
pkg: zhex/internal/lang
cpu: AMD Ryzen 7 6800H with Radeon Graphics         
BenchmarkTextStatsCounts-16     	     295	   8353699 ns/op	14938052 B/op	     266 allocs/op
BenchmarkTextStatsUpper-16      	     247	   9583915 ns/op	 5171638 B/op	   94223 allocs/op
BenchmarkCompileTextStats-16    	  168361	     14676 ns/op	   26360 B/op	     211 allocs/op
BenchmarkFibonacci-16           	   97290	     24969 ns/op	   48760 B/op	     219 allocs/op
PASS
ok  	zhex/internal/lang	11.946s
```

Second run, same flags:

```text
goos: linux
goarch: amd64
pkg: zhex/internal/lang
cpu: AMD Ryzen 7 6800H with Radeon Graphics         
BenchmarkTextStatsCounts-16     	     277	   8242630 ns/op	14938069 B/op	     266 allocs/op
BenchmarkTextStatsUpper-16      	     254	   9647900 ns/op	 5171636 B/op	   94223 allocs/op
BenchmarkCompileTextStats-16    	  175026	     13895 ns/op	   26360 B/op	     211 allocs/op
PASS
ok  	zhex/internal/lang	9.142s
```

Reproducibility: two fresh runs of the same command on the same machine reproduced the same shape — identical allocs/op (266 / 94,223 / 211 / 219), B/op within single digits, and times in the same bands (counts 8.5–8.8 ms, upper 9.3–9.8 ms, compile 13.5–14.2 µs, fibonacci 23.9–24.4 µs).

## Stability

Allocation counts are exactly stable across runs for every benchmark — 266, 94,223, 211, and 219 — and B/op varies by at most 17 bytes. Time noise stays within expectations: upper is steadiest (9.58 vs 9.65 ms, 0.7%), counts moves 8.35 vs 8.24 ms (1.3%), and compile is noisiest (14.68 vs 13.90 µs, ~5%), as expected for a microbenchmark. Fibonacci was sampled once per run.

## Findings

- Checking is cheap next to running: compiling the program costs about 14 µs, roughly 0.2% of the 8.3 ms counts run. The checker is not the bottleneck.
- Counts mode allocates 14.9 MB in only 266 allocations — whole-body string work dominates the bytes: few, large allocations as the file's text is copied and rescanned.
- Streaming upper allocates 5.2 MB across 94,223 allocations, about 4.7 per line — small, steady allocations. The two modes differ in per-op cost profile, not just in totals: streaming trades allocation count for less than half the memory.
- The fibonacci baseline is 25 µs for recursive fib(20) through the tree-walking interpreter, a sense of scale for the 8–9 ms text passes.

## Readability review

The utility itself is small — 11 lines, 60 words, 467 chars — but the two header comments are 136 chars, 29% of the file. The counts print on line 8 is 158 chars, 34% of the whole file in one statement: 3 `int.to-string` calls, 3 `text.*` counting calls, 5 `+` operators, and 3 label fragments, where only about 4 of its ~25 tokens carry the actual measurement. Mode dispatch is by arity (`console.args-count() == 1`), not by flag value — the second argument's content is never read, and `"upper"` is a convention between the comment and the caller. The branches are asymmetric: upper is two short lines (63 chars total); counts is one 158-char statement. Argument handling is a single shared line, `let path = console.arg(0) try`; both modes inherit it, and a missing argument or unreadable file exits with a diagnostic before either branch runs — the double spaces in `" lines  "` and `" words  "` are deliberate column alignment. The split shows in cost too: counts pays in bytes, streaming in allocation count (see Findings).

## Recommendation

The director's recommendation, recorded verbatim here and on issue #4; the choice stays with the owner.

> Recommendation for the next application: a line-filter tool (grep-adjacent) — `zhex run filter.zhex <pattern> <file>` printing matching lines. Grounds: (1) the measurements show the streaming path is the memory-efficient shape (5.2 MB vs 14.9 MB for whole-body work) and a filter is naturally streaming; (2) it exercises exactly the shipped vocabulary (contains/index-of/trim-start-style Tier-2 words) and motivates the Tier-2 batch; (3) it is the first tool an agent reaches for on logs, matching the language's agent-first direction. Separately — not part of this ticket — the readability review flags number-to-text ceremony (`print(int.to-string(a) + " " + int.to-string(b))`, the 158-character counts line) as the top formatting cost; a future language story should consider string interpolation or a lighter join. The choice stays with the owner.
