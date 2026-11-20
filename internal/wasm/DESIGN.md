# Zhex → WebAssembly, Milestone 1: design contract

Binding inputs: `docs/wasm-plan.md` decisions D1–D7, ticket #92, `AGENTS.md` (voice, tests-first).
This document pins every interface three parallel builders share. Where a section contradicts
`docs/wasm-plan.md`, the Surprises section below is the reason and this document wins for M1.

## Surprises (read first)

1. **wat2wasm cannot assemble WasmGC.** Verified on the installed wabt 1.0.42 (2026-11-14):
   `(type $s (struct ...))` parses under `--enable-gc`, but every GC *instruction*
   (`struct.new`, `array.get`, `ref.i31`, …) fails with `unexpected token, expected an instr`,
   even with `--enable-all`; the binary contains no GC mnemonics — wabt never shipped full GC
   text support. The ticket's "WasmGC assembly confirmed" only exercised types.
   **Decision:** D2's route (hand-written WAT text + external assembler, no binary encoder in Go)
   stands, but the assembler is `wasm-tools` (BytecodeAlliance `wat` crate — same family as the
   installed wasmtime 50; on Arch via `pacman -S wasm-tools`, elsewhere `cargo install wasm-tools`).
   Invocation: `wasm-tools parse <in.wat> -o <out.wasm>` assembles the binary but does **not**
   validate it (verified 2026-11-15: parse exits 0 on type-invalid modules, e.g. an i64 returned
   where i32 is declared), so the pipeline runs `wasm-tools validate <out.wasm>` as a separate
   step and treats a nonzero exit as a compile failure. No feature flags needed. The entire
   runtime idiom below was assembled with wasm-tools 1.263 and executed
   correctly under Node 22.23: imports, GC structs/arrays, mutable ref fields, i31 boxing,
   depth-counter trap mapped back through an exported error code.
2. **The checked tree is package-private.** `internal/lang` exports `CompileModules` but every
   node type (`word`, `statement`, `expression`, `binding`) is unexported, so a package under
   `internal/wasm/` cannot walk it. **Decision:** the WAT emitter lives in `internal/lang`
   (files `wat.go`, `watnames.go`, `watprelude.go`) next to `eval.go`, which it mirrors; one new
   exported function `lang.CompileWat`. `internal/wasm/` keeps orchestration: assembler exec,
   the CLI entry, and all tests/fixtures.
3. **Zhex `Array<T>` has rank 1–32** (`collections.go arrayShape`); a rank-1 array is a
   one-dimensional table. Arrays therefore need a shape wrapper, not a bare GC array.
4. **Lists are reference values and `list.append` mutates the shared list**, so a bare GC array
   breaks aliasing on growth. Lists are a wrapper struct with a mutable items field (below).

## 1. Package layout

```
internal/lang/watnames.go     A  constants: prelude type/function names, fail codes, name mangling
internal/lang/watprelude.go   B  const watPrelude string — fixed types, memory, globals, import, helpers
internal/lang/wat.go          A  func CompileWat(entryPath, entrySource string,
                                     resolve func(string) (ModuleSource, bool)) (string, error)
internal/lang/wat_test.go     A  emitter unit tests (construct → key WAT lines; fence cases)
internal/wasm/compile.go      C  func Compile(path, source, outPath string,
                                     resolve func(string) (lang.ModuleSource, bool)) error
                                   + unexported assemble(wat, outPath) error (LookPath + exec)
internal/wasm/compile_test.go C  differential + diagnostic + missing-tool tests (skip-safe)
internal/wasm/testdata/       C  fixtures *.zhex and run.mjs (the Node runner)
cmd/zhex/main.go              C  `compile` subcommand on the existing run(args, in, out, diag) seam
```

`lang.CompileWat` reuses `CompileModules` (check first; its error returns unchanged, so the CLI
renders checker diagnostics exactly as `run` does) then emits the module text. `internal/wasm`
never imports the tree — only the string. The new fence code (§6c) goes into `codes.go` +
`catalog.md` (codes_test enforces the sync).

## 2. Value representation (WasmGC)

Fixed types (owned by the prelude, always emitted; `X` ∈ {i64, i32, ref} specialization):

```wat
(type $zhex_string      (array (mut i32)))                       ;; Unicode code points
(type $zhex_arr_i64     (array (mut i64)))
(type $zhex_arr_i32     (array (mut i32)))
(type $zhex_arr_ref     (array (mut (ref null eq))))
(type $zhex_list_i64    (struct (field $items (mut (ref null $zhex_arr_i64)))) (field $active (mut i32))))
(type $zhex_array_i64   (struct (field $shape (ref $zhex_arr_i64))
                                (field $items (mut (ref null $zhex_arr_i64))) (field $active (mut i32))))
(type $zhex_some_i64    (struct (field $value i64)))
(type $zhex_none        (struct))
(type $zhex_result      (struct (field $tag i32) (field $payload (ref null eq))))
```

- **Int = i64** unboxed; **Bool = i32** 0/1 unboxed (only Int/Bool/String compare or print).
- **Unit**: no value — placeholder `i32 0` in parameter positions, `(ref.null eq)` in reference
  positions; Unit-result words have no wasm result.
- **String = `$zhex_string`**, an array of i32 code points (literals decode exactly as the Zhex
  lexer did). `print` stages UTF-8 bytes into a prelude-owned linear memory and imports
  `env.print(ptr, len)`; the JS side decodes with `TextDecoder` (§4). No GC refs cross to JS.
- **List<T>/Array<T>**: element-representation specialization — `i64`, `i32` (Bool), `ref`
  (everything else) — so Int lists are never boxed. A list is the wrapper struct (`list.new` =
  wrapper with an empty items array); `array.new` checks rank/extent/limit at runtime and fills
  the wrapper's shape + items. Both wrappers carry `$active` for iteration-mutation checks.
- **Choices**: one struct per variant (`$zt_<mangled-type>_<Variant>`) holding that variant's
  payload fields in declaration order; payload-less variants get a fieldless struct.
  (Amended 2026-11-15: WasmGC declared types are **structural** — same-shape variants
  canonicalize to one type, so `ref.test` cannot distinguish them and plain struct-per-variant
  is insufficient. The emitter therefore pads variant *i* with *i* i64 fields ahead of the
  payload plus a trailing i32 discriminant, making every variant's shape unique.)
  **Option** = `$zhex_some_X` per payload repr + one shared `$zhex_none`.
- **Result** = the single `$zhex_result` struct: tag 0 ok / 1 err, payload boxed with
  `(struct.new $zhex_some_i64 v)` for Int payloads (amended 2026-11-14: `ref.i31` holds 31
  bits and corrupts |v| ≥ 2^30; unbox = `ref.cast (ref $zhex_some_i64)` +
  `struct.get $zhex_some_i64 $value`), `ref.i31` for Bool payloads, direct ref otherwise,
  null for Unit. One type suffices because `err()` is unconstructible in the M1 subset
  (`error.new` is out of scope, and no fallible builtin exists in scope), so the *err*
  payload is never inspected.
- **Records** are included (they fall out of the choice representation trivially): struct with
  fields in declaration order; positional and all-named construction (the checker has already
  resolved `argFields`), field reads via `struct.get` chains. Record field defaults are fenced.

Name mangling (injective; prefix then per-char escape: `_`→`_u`, `-`→`_h`, `.`→`_q`):
words `$zw_<mangled>` (`math.gcd` → `$zw_math_qgcd`), user types `$zt_<mangled>`.
Locals: `$l<n>_<name>` with an emitter-assigned unique index (bindings can shadow).

## 3. Word calling convention

- One wasm func per word: `(func $zw_name (param $p0 repr ...) (result repr?))`, parameters and
  result in §2 representation. Direct calls only; no funcrefs in M1.
- Every binding (`let`/`var`/loop/`match` arm) becomes one wasm local keyed by the tree's
  `*binding` pointer; `var` assignment is `local.set` on the same local. Params bind their locals
  on entry. Shadowing is two locals — the pointer is the identity.
- Body value = last statement's value, mirroring `eval.block` (expression/if/flow/match produce
  values; let/for/assign produce Unit). Statements emit stack-neutral; the consumed value of a
  block tail is stored into a result local. `return` stores the value and branches out.
- **No `return` instruction anywhere.** Each word body is wrapped in `(block $zhex_ret ...; br
  $zhex_ret)`; the single epilogue runs `call $zhex_leave` then yields the result local, so the
  depth counter is always decremented. `$zhex_main` (top-level statements) follows the same shape
  minus enter/leave.
- **Recursion depth**: prelude global `$zhex_depth` (mut i32). `$zhex_enter` mirrors eval
  exactly — `depth >= 256` → `$zhex_fail(2)`, else increment — called as the first instruction of
  every user word; `$zhex_leave` decrements, once, in the epilogue. Builtins and `$zhex_main`
  don't count (interpreter parity; verified: fib(300) trips the counter, not the native stack).

## 4. Imports

Exactly one import, in every emitted module:

```wat
(import "env" "print" (func $env.print (param i32 i32)))
```

`ptr`/`len` address UTF-8 bytes staged in the exported linear memory. The JS runner provides
`env.print(ptr, len)` decoding from `instance.exports.memory.buffer` (read per call — growth
detaches the buffer), appending `\n` (the interpreter's `print` writes `text + "\n"`). Exports:
`"memory"`, `"zhex_main"` (§7 — no start section; the runner calls it, so traps land in JS),
`"zhex_last_error"` (§6a).

## 5. Runtime prelude

A single Go raw-string constant `watPrelude` in `internal/lang/watprelude.go` — a fixed hand
written WAT text, not generated per program — emitted verbatim ahead of the program's WAT.
Inventory:

- `$zhex_fail(code i32)` — sets `$zhex_err` to the code, then `unreachable` (the only deliberate trap).
- `$zhex_enter` / `$zhex_leave` — depth counter (§3).
- `$zhex_int_to_string(i64) (ref $zhex_string)` — decimal, exactly Go's `strconv.FormatInt`.
- `$zhex_str_eq(a, b (ref $zhex_string)) i32` — length + code-point compare for `==`/`!=`.
- `$zhex_str_concat(a, b) (ref $zhex_string)` — for String `+`.
- `$zhex_print_str((ref $zhex_string))` — UTF-8-encodes into memory at offset 0 (growing pages), calls `env.print(0, len)`.
- `$zhex_print_int(i64)` — int_to_string then print_str; Bool prints constant `"true"`/`"false"`.
- `$zhex_list_new_X()`, `$zhex_list_append_X(list, elem)` (shared-reference growth: allocate a
  fresh array, store into `$items`; checks `$active` → fail 10, 1,000,000 limit → fail 8),
  `$zhex_list_length_X`, `$zhex_list_get_X/set_X` (bounds → fail 3, set also checks `$active`
  → fail 10).
- `$zhex_array_new_X(shape list, fill) (ref $zhex_array_X)` — rank 1–32 → fail 5, negative
  extent → fail 6, element limit → fail 9, zero extent → empty items; row-major flat items.
- `$zhex_array_get_X/set_X(arr, coords (ref $zhex_arr_i64))` — rank match → fail 7, per-axis
  bounds → fail 4, offset = `offset*shape[i] + coord[i]` (eval order), set checks `$active`.
- Small helpers as needed (e.g. building an array loop's coords list).

## 6. Diagnostics at runtime

**(a) Deliberate traps.** Every Zhex runtime failure calls `$zhex_fail(code)`; codes (in
`watnames.go`) and the exact stderr strings the Node runner prints (then `process.exit(1)`):

| code | Zhex code | runner prints (exact, one line, stderr) |
|---|---|---|
| 1 | integer-overflow | `runtime error: integer overflow` |
| 2 | depth-limit | `runtime error: word call depth limit of 256 exceeded` |
| 3 | list-range | `runtime error: list index is out of range` |
| 4 | array-range | `runtime error: array coordinate is out of range` |
| 5 | array-rank | `runtime error: array rank must be between 1 and 32` |
| 6 | array-shape | `runtime error: array extents cannot be negative` |
| 7 | array-rank | `runtime error: array index has the wrong rank` |
| 8 | collection-limit | `runtime error: list exceeds element limit` |
| 9 | collection-limit | `runtime error: array exceeds element limit` |
| 10 | collection-iterating | `runtime error: collection is being iterated` |
| 11 | unhandled-result | `runtime error: unhandled failure` |
| 19 | division-zero | `runtime error: division or remainder by zero` |

Messages mirror `eval.go`/`collections.go` phrasing; the `runtime error: ` prefix replaces the
interpreter's `file:line:column:` position, which the artifact does not carry in M1. After
catching a `RuntimeError` the runner reads `zhex_last_error()`; 0 means a native trap.

**(b) Arithmetic failures.** Division/remainder by zero explicitly calls fail 19,
so the WASI adapter flushes earlier stdout and prints the same diagnostic as the
browser runner before exiting 1. The runner retains its native-zero-trap mapping
for older artifacts.
`MinInt64 / -1` gets an **explicit** check → fail 1 (native wording "divide result unrepresentable"
varies by engine; the interpreter calls it integer overflow). `MinInt64 % -1` returns 0 natively.

**(c) Compile-time fence.** Out-of-scope constructs (any builtin not: `print`, `int.to-string`,
`ok`, `err`, `list.new/append/length`, `array.new/length`; `while`; `for line in file.stream`;
`for` over maps; `try`/`catch` directly on collection ops (checked form); function
values/partials; word or record field defaults) fail emission with a normal positioned lang
diagnostic, new code `CodeNotAvailableInBuild Code = "not-available-in-build"` (add to `codes.go`
+ `catalog.md`): message `%s is not available in this build` and hint
`this construct is outside the wasm milestone-1 subset; run it with zhex run instead`.
(Amended by DESIGN-M2 §6: the hint is milestone-neutral — "this construct is not
compiled by zhex compile yet; run it with zhex run instead".)

**(d) Overflow checks** mirror `eval.binary` exactly: `+` overflow iff `b>0 && a>Max-b ||
b<0 && a<Min+b`; `-` iff `b<0 && a>Max+b || b>0 && a<Min+b`; `*` iff `a!=0 && (a==-1 && b==Min ||
n/a != b)` with the `n/a` division guarded behind the `a==-1` test (wasm `||` does not
short-circuit; the unguarded `Min/-1` traps). Unary `-` on `Min` → fail 1.

## 7. Emission approach

- `wat.go` walks the checked tree with one method per node kind, mirroring `eval.go`'s switch
  structure (`expression` kinds: int/bool/string/name/binary/unary/call/list/index/index-write/
  match/try/catch/statement-value; statement kinds: expression/return/let/var/assign/if/for/
  for-collection/flow). Anything else → fence (§6c). WAT is written immediately into a
  `strings.Builder` with a small indent helper — no intermediate IR. Output is deterministic
  (declaration order everywhere; locals numbered in visit order).
- `and`/`or` short-circuit via `(if (result i32))` on a tee'd left operand. `if`/`elif`/`else`
  as statement or value: `(block ...)`/`(if (result T))`; a `match` is a `ref.test` if-chain in
  source arm order (checker guarantees exhaustiveness; `case _` is the else), payload bindings
  are `struct.get`s. `for n in range`: plain i64 counter loop (empty when `start >= end`).
  `for v in list|array`: set `$active`, loop, reset; array dual-binding builds the coords list.
  `flow`: current value in a local; each step is the call with the value as first argument plus
  `step.args[1:]` (the tree's `args[0]` is the incoming-value slot, as in eval). `try`: payload
  when tag 0, else fail 11; `catch`: payload when tag 0, else handler with the binding bound to
  the opaque result struct. (Amended by DESIGN-M2 §2d: the catch binding holds the
  err payload, not the Result struct.) Binary `+` on String renders Int operands via `$zhex_int_to_string`
  first (the string-context Int exception in eval).
- Module assembly order: `watPrelude` (fixed types, memory, globals, import, helpers), then
  program-specific types (choice variants, records), then word funcs in declaration order, then
  `$zhex_main` for `p.body`, then the three exports. No `(start)` section — the runner calls
  `zhex_main()` explicitly so imports can close over `instance.exports` and traps land in JS.
- **Assembly**: `internal/wasm.assemble` writes the WAT to `os.CreateTemp("", "zhex-*.wat")`,
  runs `wasm-tools parse <tmp.wat> -o <outPath>` then `wasm-tools validate <outPath>`
  (LookPath first, §8; parse assembles without validating — Surprises 1), removes the temp; a
  non-zero exit from either step wraps the tool's stderr as
  `wasm-tools failed: <stderr>` (it is a compiler bug).

## 8. CLI contract

`zhex compile <file.zhex> [-o <out.wasm>]` on the existing `run(args, …)` seam in
`cmd/zhex/main.go`; usage string gains the subcommand. First non-flag arg is the file; `-o`
sets the output; default output = input path with `.zhex` replaced by `.wasm` (append if the
suffix differs). No other flags in M1 (no `--report`).

- **Checker errors in the source**: returned before anything is emitted or written; the CLI
  prints them with the existing renderer (position, excerpt, caret, hint) to diagnostics and
  exits 1. No output file is created.
- **Missing tool** (`exec.LookPath("wasm-tools")` fails): print exactly these two lines to
  diagnostics, exit 1, no output file:
  ```
  wasm-tools is not installed; zhex compile needs it to assemble the WebAssembly module (code: missing-tool)
  hint: install it — cargo install wasm-tools (rustup), pacman -S wasm-tools (Arch), or brew install wasm-tools (macOS)
  ```
  (`wat2wasm` cannot assemble WasmGC — see Surprises 1 — so the diagnostic names `wasm-tools`.)
- Usage errors (missing file, unknown flag) exit 2 like `run`. Success exits 0.
- `internal/wasm.Compile(path, source, outPath, resolve)` returns `nil` only with the file
  written; the CLI passes its existing `readModule` sibling resolver, so `use` modules compile.

## 9. Test strategy

- **Emitter unit tests** (`internal/lang/wat_test.go`): per construct, compile a small program
  with `CompileWat` and assert the WAT contains the load-bearing lines (e.g. `call $zw_fib`,
  `i64.add`, `$zhex_enter`); fence tests assert `not-available-in-build` for `text.reverse`,
  `while`, checked `list.append … catch`. No tools needed.
- **Differential test** (`internal/wasm/compile_test.go`): fixtures in `testdata/` —
  `fib.zhex` (recursion + `flow` + prints), `core.zhex` (var, `for range`, `if/elif/else`,
  comparisons, and/or/not, negative numbers), `lists.zhex` (`list.new/append/length`, index
  write, `for value in list`, String `+`, `int.to-string`), `choices.zhex` (user choice +
  `match`, `Option.Some/None`, a record). Per fixture: run the interpreter
  (`lang.Run` into a buffer) and the compiled module (`node testdata/run.mjs <file.wasm>`),
  compare stdout **byte for byte** and the exit status. `t.Skipf` when `wasm-tools` or `node`
  is missing (`exec.LookPath`) so tool-less CI stays green; `just test` (plain `go test ./...`)
  picks this up unchanged.
- **Runtime diagnostic tests**: `overflow.zhex` and `depth.zhex` compile and the Node run's
  stderr equals the exact §6a strings with exit 1 (compiled-only assertions; the interpreter's
  rendering carries positions and is already covered by lang tests).
- **Missing-tool test**: `t.Setenv("PATH", "/nonexistent")` and assert the exact §8 two-line
  message and exit path.
- `run.mjs` is normative for the import contract: instantiate with `env.print` (TextDecoder,
  `\n` append), call `zhex_main()` in try/catch, map traps per §6a/§6b table (embedded), print
  collected stdout, exit 0/1 accordingly.

## 10. Default choices taken

- Assembler = `wasm-tools` (D2 said wat2wasm; impossible — Surprises 1). Node ≥ 22 assumed where tests run.
- Emitter in `internal/lang` (tree is unexported — Surprises 2); `internal/wasm` = orchestration + tests.
- String = GC array of i32 code points; print crosses to JS as UTF-8 bytes in linear memory via
  `env.print(ptr, len)` — no GC refs cross the boundary.
- Collection element specialization (i64/i32/ref array types + list/array wrapper structs per
  repr); no boxing except Result payloads (i31 for Bool; Int payloads box as `$zhex_some_i64` —
  the 31-bit i31 corruption was found during prelude review, §2 amendment).
- Choice = struct-per-variant, match = `ref.test` chain in source order; `Option` via per-repr
  `Some` + shared `None` (variants padded to distinct shapes because WasmGC types are
  structural — §2 amendment). All-named record constructions mint a unique temporary id per
  construction (fixed 2026-11-15: a shared id collided across differently-typed constructions
  in one word, feeding a value to a local of the wrong type).
- Result = one `{tag, eqref}` struct; `try`-on-err traps fail 11 (defensive — `err()` is
  unconstructible in the M1 subset because `error.new` is fenced); `try/catch` on collection
  ops (checked form) fenced.
- Records included (positional + named construction, field reads); record *defaults* fenced.
- `int.to-string` included (falls out of print's int formatting).
- `MinInt64 / -1` explicitly checked (native wording differs); `%` relies on native semantics.
- No `(start)` section (the runner calls exported `zhex_main` so import closures get the
  instance); no `--report` on `compile` in M1.
- Prelude is one Go string constant, always emitted in full (no dead-helper pruning).
- Word param defaults, `while`, map iteration, function values — fenced; `range` is loop-only.
- `missing-tool` is a CLI-rendered message, not a lang catalog code; `not-available-in-build` is.
- Fail-code table and exact runner strings pinned in §6a — tests assert those literals.

## 11. Builder work-split

- **A — emitter core**: `internal/lang/wat.go`, `internal/lang/watnames.go` (check this in
  first — B and C compile against its constants), `internal/lang/wat_test.go`. Owns the tree
  walk, mangling, locals, call convention, fence.
- **B — runtime prelude**: `internal/lang/watprelude.go` (self-contained WAT text; develops
  against §2/§5/§6 which pin every name it uses; sanity-check by assembling a hand-written
  module around it when `wasm-tools` is present, skip-safe otherwise).
- **C — CLI, assembly, differential harness**: `internal/wasm/compile.go`,
  `internal/wasm/compile_test.go`, `internal/wasm/testdata/*` (fixtures + `run.mjs`),
  `cmd/zhex/main.go` compile branch. Owns §8 messages and §9 tests.
- Shared interfaces are exactly: `lang.CompileWat` (A→C), `watnames.go` constants (A→B),
  the §2 type names + §5 helper names + §6a code table (A↔B), and `internal/wasm.Compile`
  (C→CLI). No builder edits another's files; the only shared existing files touched are
  `cmd/zhex/main.go` (C), `internal/lang/codes.go` + `catalog.md` (A adds the fence code).
