# WebAssembly: run Zhex in WASM, then compile Zhex to WASM

There are two different WASM stories and they are frequently conflated. This document separates them, records the findings verified on 2026-11-14, and lays out the detailed plan for [#7](https://github.com/desireco/zhex/issues/7) — compiling Zhex programs to WASM. All decisions in Part 5 were ratified in the owner review on 2026-11-14.

- **Track A — interpreter in WASM.** Compile the existing Go interpreter to a wasm module; Zhex source goes in, output comes out. Ships the browser playground. Days to weeks.
- **Track B — Zhex to WASM.** Compile Zhex *programs* to standalone `.wasm` artifacts (the ROADMAP Future item, #7). Months.

The tracks stack: A's browser shell and site UX survive unchanged when B's engine replaces A's interpreter.

## Part 1 — Verified findings (2026-11-14)

All checked against the working tree; nothing below is speculation.

- `GOOS=js GOARCH=wasm go build ./...` and `GOOS=wasip1 GOARCH=wasm go build ./...` both compile **clean, zero changes**. The one raw syscall (`file.free-space` → `syscall.Statfs`) is already isolated behind build tags (`internal/lang/freespace_unix.go` / `_other.go` / `_windows.go`).
- Full CLI as a wasm module: **7.1 MB raw, 1.9 MB gzipped**. That is the Go runtime floor; fine for a lazy-loaded "Try it" button, wrong for an eager script tag. TinyGo could shrink it 5–10× but risks the interpreter's runtime use — rejected for v1.
- Runtime proof under Node (`wasm_exec.js`): the module boots, parses args, writes stdout (the REPL banner prints), and produces the correct `not implemented on js` diagnostic when asked to open a file. The only missing piece is an entry point that takes **source as a string instead of a file path**.
- The seam is already clean: `run(args, io.Reader, io.Writer, io.Writer)` at `cmd/zhex/main.go:29` takes IO injected; the only `os.ReadFile` calls in the CLI load the source itself.
- Compiler-side assets: the checker (`internal/lang/check.go`, 2,081 lines) is fully reusable unchanged; `eval.go` (797 lines) is the size reference for an emitter; `examples/*.zhex` is a ready-made differential test suite.

## Part 2 — Why Zhex compiles cleanly

The properties that make language backends hard are, mostly by design, absent:

- **Checked and monomorphic.** The checker assigns one concrete type to every expression before anything runs. No generics, no implicit conversions, no dynamic dispatch, no reflection. The checked tree tells the emitter exactly what to emit.
- **No floating point.** The language is `Int`-only, and `Json.Number` carries its literal as a `String`. The entire float-formatting/dtoa class of wasm pain does not exist here.
- **No concurrency.** No threads means no wasm atomics, no shared memory, no memory-model questions.
- **Recursion is bounded (256).** Depth handling is already a language diagnostic, not a wasm trap to explain after the fact.
- **Closed surface.** Nominal choices with checked-exhaustive `match`, a fixed ~110-word builtin list, `Option`/`Result` built in.

## Part 3 — Where the work lives (ranked)

1. **Runtime library — the most code.** WASM gives arithmetic and memory; Zhex's promises on top must be written: the UTF-8 `text.*` suite (the biggest chunk with temporal gone), a hash map for `Map`, integer formatting, buffer semantics.
2. **Data representation — the most thinking.** WasmGC vs linear memory + own GC (Part 5, D1).
3. **Codegen mechanics — tame for Zhex.** `Int` → `i64` (division by zero traps natively; **overflow needs explicit checks** since the language promises it is an error). Records → GC structs, choices → tagged refs, function values → typed funcrefs. Runtime errors must format as Zhex diagnostics through a runtime call — never surface as raw wasm traps.
4. **Host contract — the most external risk.** `file.*`, `read-line`, buffers need imports (JS imports for browser, WASI for standalone). `docs/forth-implementation-research.md` already names this correctly: the instruction format is not the work; the host API is.

## Part 4 — Track A plan (interpreter in WASM)

| Step | Work | Effort | Ticket |
|---|---|---|---|
| A1 | Wasm entry point: build-tagged `main` exposing `runSource(source, stdin) → {output, report, trace}` via `syscall/js`; unsupported words (`file.*`, `read-line`, `console.arg`) return coded diagnostics | 1–2 days | [#91](https://github.com/desireco/zhex/issues/91) |
| A2 | Website playground: editor pane, run button, output pane, diagnostics with carets (renders `--report=json` + spans client-side), example preloading, lazy-load behind a click | 1–2 weeks, mostly site/UX |
| A3 | WASI build as a sandboxed runner (preopen one directory; `file.*` words work against it). Compiles today; runtime untested — no wasm runtime installed locally | a few days, after installing wasmtime |

Track A is ticketed: the engine itself is [#91](https://github.com/desireco/zhex/issues/91) (A1), and the site demos that consume it are the browser playground [#88](https://github.com/desireco/zhex/issues/88), live docs snippets [#89](https://github.com/desireco/zhex/issues/89), and the Game of Life demo [#90](https://github.com/desireco/zhex/issues/90). The playground is the top of the funnel per `docs/language-presentation-research.md` (Rust and Gleam both treat the browser trial that way) and it demos the diagnostics thesis directly.

## Part 5 — Decisions (ratified 2026-11-14)

- **D1 GC strategy — WasmGC.** Garbage-collected structs/arrays inside the wasm engine (all evergreen browsers since ~late 2024; how Go/Kotlin/Dart ship now). No collector to write; the trade — no pre-2024/older-embedded engines — was accepted.
- **D2 Emission route — WAT text + wabt.** Emit WebAssembly text, assemble with `wat2wasm`; debuggable, no binary-encoder code. Hand-emitted binary only if the external tool dependency ever hurts. The C route (`clang --target=wasm32`, already installed) stays deferred to the native-backend story.
- **D3 Temporal — Duration-only.** `duration.` ships in the compiled target (pure computation, no database). `date.`/`time.`/`instant.`/`timezone.`/`zoned.` stay interpreter-only in v1; using them in a compiled program is a coded not-available-in-this-build diagnostic. The tz database stays out of every artifact.
- **D4 Host contract — browser first, WASI at M3.** Browser: JS imports; unsupported words fail with stable diagnostic codes (same family as A1). Standalone arrives at M3 as WASI with preopened directories, `file.*` against the sandbox only. Two import skins, one runtime.
- **D5 Sequencing — independent tracks.** No ordering constraint between Track A (#91) and Track B (#7); the board sets priority ticket by ticket.
- **D6 CLI surface — `zhex compile`.** `zhex compile file.zhex -o file.wasm` is the public command; pairs with `zhex run` (check + interpret). The name is decided now; the subcommand ships with M1's first usable output and follows the vocabulary checklist when it does.
- **D7 Ticket structure — per-milestone tickets.** #7 is the umbrella (plan, decisions, artifacts, final close); each milestone M1–M4 becomes its own ticket when it starts, so no ticket sits In Progress unattended across the ~2–3 month arc.

## Part 6 — Track B milestones (#7)

Differential testing is the acceptance method throughout: the same program runs through the interpreter and the compiled artifact; outputs must match **byte for byte**. The `examples/` corpus is the test suite. Per D7, each milestone becomes its own ticket when it starts; #7 is the umbrella.

| Milestone | Scope | Acceptance | Effort |
|---|---|---|---|
| **M1 — first light** | `Int`/`Bool`/`String`/`Unit` arithmetic and comparisons, `let`/`var`/`if`/`match`, words + recursion (depth counter), `flow`, `for`/`range`, `List`/`Array` as GC arrays, `Option`/`Result`, `print` via import | `fib`-class programs run in a browser with output identical to the interpreter | 2–4 weeks |
| **M2 — text and maps** | UTF-8 `text.*` suite, `list.*` callbacks via typed funcrefs, `Map` (hash map in the runtime), `sort`/`sort-by` | corpus programs without file access (e.g. `times-table`, `edit-distance`) differential-pass | 2–3 weeks |
| **M3 — host interface** | JS imports + WASI for `file.*`/`read-line`/`buffer.*` within D4; unsupported-word diagnostics with stable codes; `duration.*` runtime (D3); `json` | `text-stats` and `file-work`-style examples pass under wasm against a sandboxed FS | 2–3 weeks |
| **M4 — parity** | Full-corpus differential gate (a `just` target, separate from `just test`), artifact size report, overflow and depth diagnostics verified | every `examples/*.zhex` produces byte-identical output, interpreter vs compiled | 1–2 weeks |

With these decisions ratified, the total is roughly **2–3 months part-time** for a credible v1.

## Part 7 — Explicitly out of scope for v1

`date.`/`time.`/`instant.`/`timezone.`/`zoned.` in the compiled target (D3 — `duration.` ships), `--trace` instrumentation of compiled code, pre-WasmGC engine support, TinyGo, the native backend (separate ROADMAP line), and optimizer work beyond what wabt's flags provide.

## Part 8 — Risks

- **WasmGC emitter immaturity in Go** — mitigated by the WAT text route (D2); no binary encoding code needed for v1.
- **Host-contract scope creep** — M3 is fenced; new host words get their own tickets.
- **Byte-identical discipline** — no floats and JSON literals-as-strings remove the classic divergence sources; string sort order and map traversal order are the remaining ones to pin in tests.
- **WASI version churn** — pin a preview level per release and record it in the artifact report.
