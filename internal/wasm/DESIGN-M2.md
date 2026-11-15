# Zhex → WebAssembly, Milestone 2: delta contract

Binding inputs: `internal/wasm/DESIGN.md` (M1 — still holds except where amended
here), `docs/wasm-plan.md` D1–D7, ticket #95, and the interpreter in
`internal/lang` (`host.go`, `eval.go`, `collections.go`, `maps.go`) at the
merge of main into `wasm-m1`. Interpreter semantics were pinned from that code,
not from memory; where wording differs, the code wins. Only the delta from M1
is specified. Acceptance stays: differential fixtures produce byte-identical
stdout and exit status vs `zhex run`; stderr follows the M1 convention (the
`runtime error: ` prefix replaces the interpreter's `file:line:column:`).

## Surprises (read first)

1. **The string-context Int exception and `print`-of-Int already ship in the M1
   emitter.** After the Step-0 main merge, `wat.go` binary `+` renders an Int
   operand via `$zhex_int_to_string` when `x.typ == stringType`, and
   `printCall` has the `intType` case. Ticket #95 lists them as core deltas;
   M2 adds fixtures only (§7). `while` is the one remaining core-delta statement.
2. **`upper`/`lower` need a Unicode case table.** Go's `strings.ToUpper`/
   `ToLower` are per-code-point *simple* mappings (1:1, never change length, no
   context rules: `Σ`→`σ` always, `ß`→`ß` under ToUpper, `ſ`→`S`). ASCII-only
   code fails the `éà` fixture. See §1.
3. **The `text.to-int` failure message quotes the *untrimmed* input with Go
   `%q`** (`host.go`: `errorAt(... "%q is not a signed 64-bit decimal integer",
   args[0].text)` — the raw text, not the trimmed one that was parsed). The err
   value therefore carries a pre-rendered message string (§2).
4. **M1's `catch` bound the whole `$zhex_result` struct; M2 must bind the err
   payload.** eval's catch binds `value{fault: v.result.err}` — the payload, not
   the Result. One small change inside `catchExpr` ships with §2.
5. **Some failures now carry dynamic text** (the to-int message, the
   array.select rank), which the M1 fixed-string fail table cannot express:
   codes 12–15 add a staged-message mechanism (§2c).
6. **Map `remove` is order-preserving shift** (interpreter `maps.go`: delete,
   `copy` left, reindex tail) and a later re-put appends at the *end*. Any
   tombstone/reuse scheme breaks insertion-order parity — pin the rebuild (§3).
7. **Map iteration is asymmetric**: single binding iterates *values*; dual
   binding binds key first, value second. Easy to get backwards (§3d).

## 1. `text.*` suite

String = `$zhex_string` (code-point array, M1 §2); all semantics below are in
the **code-point domain**, which is observably identical to the interpreter's
UTF-8 byte domain (UTF-8 is self-synchronizing and order-preserving). Every
word is a prelude call `$zhex_text_<op>` except `text.length`, which the
emitter inlines as `array.len; i64.extend_i32_u`. Specialization: none needed —
all operands are `$zhex_string`; `text.split` returns a ref-spec
`List<String>`, `text.join` takes one.

| word (arity) | semantics pinned from `host.go` / Go stdlib |
|---|---|
| `text.reverse(t)` | reverse the code-point array in place (fresh array); combining marks are not recomposed |
| `text.length(t)` | code-point count (`utf8.RuneCountInString`) |
| `text.slice(t, start, end)` | clamp `start<0→0`, `end>len→len`; `start>=end` → empty; else code points `[start,end)` |
| `text.index-of(t, part)` | first occurrence position in code points; absent → `-1`; empty `part` → `0` |
| `text.contains(t, part)` | substring at any offset; empty `part` → true |
| `text.upper(t)` / `text.lower(t)` | per-code-point simple case mapping (Surprises 2): a delta-encoded range table generated offline from Go's `unicode.CaseRanges` (both directions), binary-searched; 1:1, length preserved |
| `text.trim(t)` | strip leading+trailing code points in Go's `unicode.IsSpace` set: U+0009–000D, U+0020, U+0085, U+00A0, U+1680, U+2000–200A, U+2028, U+2029, U+202F, U+205F, U+3000 (exactly 25 code points) |
| `text.split(t, sep)` | `sep==""` → one part per code point (empty text → empty list); else `strings.Split` semantics: leading/repeated/trailing empty fields kept. Limit check *before* materializing: parts = (`sep==""` ? cp-count : occur+1) > 1,000,000 → fail 14 |
| `text.join(values, sep)` | plain join; empty list → `""`; no trailing separator |
| `text.replace(t, old, new)` | `old==""` → fail 13; else replace every non-overlapping literal occurrence, scanning left to right; the replacement is not rescanned |
| `text.starts-with(t, pre)` / `text.ends-with(t, suf)` | literal prefix/suffix; empty → true |
| `text.line-count(t)` | `""`→0; else count of `\n` plus 1 when there is no trailing `\n` |
| `text.word-count(t)` | `strings.Fields`: maximal runs split on the §1 trim set; `""`→0 |

`text.to-int` is §2. Checking (arity/types) is unchanged checker behavior; the
emitter just gains the builtin cases — no new checker surface.

## 2. `text.to-int` and the err value

**(a) Parsing.** Trim the §1 whitespace set from both ends, then `strconv.
ParseInt(base 10, 64)`: optional single `+`/`-`, then one or more ASCII digits
`0-9` (no underscores, no non-ASCII digits), value within Int64 — accumulate
with the same mul/add overflow checks as `$zhex_mul`/`$zhex_add`. `"  12  "`→12,
`"+7"`→7, `"007"`→7, `"9223372036854775808"`/`""`/`"1_2"`/`"0x1"` fail. Prelude
`$zhex_text_to_int((ref $zhex_string)) (ref $zhex_result)`: success = tag 0,
payload boxed `$zhex_some_i64` (M1 §2 boxing); failure = tag 1, payload = the
err struct below.

**(b) Err value.** New prelude type:

```wat
(type $zhex_err (struct (field $family i32) (field $message (ref null $zhex_string))))
```

`$family` 1 = ParseError (only family in M2; constants grow later). `$message`
is the full first-line message *without* position: `<quote(t)> is not a signed
64-bit decimal integer` where `quote` mirrors Go `%q`: wrap in `"`, emit `\"`,
`\\`, `\a\b\f\n\r\t\v`, `\xHH` for other control code points < 0x20 or 0x7F;
**shortcut: every non-ASCII code point is treated as printable and emitted
literally, upgrade when a fixture quotes non-printable non-ASCII input** (Go
would say `\u00a0` for NBSP). The `err()` builtin keeps its M1 shape (payload =
whatever was passed; unconstructible while `error.new` is fenced).

**(c) New fail codes** (append to the M1 §6a table; `watnames_m2.go` owns them):

| code | Zhex code | message | runner prints (stderr, exit 1) |
|---|---|---|---|
| 12 | text-to-int | dynamic | `runtime error: <decoded message>` |
| 13 | replace-empty | fixed | `runtime error: text.replace old text is empty` |
| 14 | collection-limit | fixed | `runtime error: text.split exceeds element limit` |
| 15 | array-rank | dynamic | `runtime error: array.select requires a rank-2 array, found rank N` |

Mechanism for 12/15: `$zhex_fail_msg(code i32, msg (ref $zhex_string))` stages
the message as UTF-8 at memory offset 0 (the same staging/growth helper
`$zhex_print_str` uses — print bytes are consumed synchronously by the import,
so the slot is free), stores the byte length in a new exported accessor
`zhex_msg_len() i32` (0 = none), sets `$zhex_err`, `unreachable`. `run.mjs`
grows the fail table with 13/14 fixed strings and, for 12/15, decodes
`memory[0..zhex_msg_len()]` and prints `runtime error: <decoded>`.

**(d) Propagation.** `try` on tag 1 inside a word: store the failed
`$zhex_result` (payload = err struct) into `$r` and `br $zhex_ret` — the M1
path, now carrying a real payload. At top level (no `$r`): read the payload's
`$message` and `$zhex_fail_msg(12, message)`; if the payload is not a
`$zhex_err` (defensive `err()` case) keep M1's fail 11. `catch` binds the
**payload** (`struct.get $zhex_result $payload`, Surprises 4) as an opaque ref;
the handler runs with it — nothing in M2 can read it (`error.*` stays fenced,
§6), so it is pass-through only. The interpreter renders the same failure as
`file:line:col: "x" is not a signed 64-bit decimal integer` (verified); the
compiled artifact drops only the position, per the M1 convention.

## 3. `Map` representation

**(a) Structure.** Maps are reference values (aliases observe one map). One
struct per key/value repr pair — key ∈ {i64, i32, String=`(ref $zhex_string)`},
value ∈ {i64, i32, ref} — named `$zhex_map_<k>_<v>`:

```wat
(type $zhex_map_i64_ref
  (struct (field $buckets (mut (ref null $zhex_arr_i32)))  ;; 64 heads, -1 = empty
          (field $next   (mut (ref null $zhex_arr_i32)))   ;; per-entry chain links
          (field $keys   (mut (ref null <k-array>)))       ;; entry keys, insertion order
          (field $vals   (mut (ref null <v-array>)))       ;; entry values, positional with keys
          (field $active (mut i32))))
```

Amendment: the sketch originally showed 4 fields; the emitted shape (see
`watprelude_m2.go`) adds `$next` because the chained hash needs a per-entry
link to the next entry in its bucket. Traversal order remains insertion order
— hash-independent, as below.

Chained hash map, fixed 64 buckets, no resizing (correctness is unaffected —
traversal order never depends on the hash; upgrade only for speed). Hash:
splitmix64 finalizer of the i64/i32 key; FNV-1a over the string's code points
then the same finalizer; bucket = `hash & 63`. Deterministic and unseeded — no
observable dependence. **There is no element limit on map growth** (the
interpreter has none; only `text.split`/list/array hit 1,000,000).

**(b) Operations** (all prelude helpers `$zhex_map_<op>_<k>_<v>`; order pinned
from `maps.go`/`host.go`):
- `put(m, k, v)` → guard first (§3e), then: existing key overwrites the value
  *in place* (keeps its position); new key appends at the end. Returns the same
  map ref (pass-through for flows).
- `m[k] = v` (index-write) = `put` with operation `map.set`; evaluation order
  receiver, key, value (M1 §7 order).
- `m[k]` (index read) = lookup → `$zhex_some_<v>` / `$zhex_none` (Option, M1
  §2); plain value, never a Result, and `try` on it is a checker error already.
- `map.remove(m, k)` → guard, then if present: allocate fresh keys/vals arrays
  of len−1, copy the prefix/suffix (shift), rebuild buckets/next entirely from
  the surviving entries in array order. Absent key: no-op. Returns Unit.
- `map.contains(m, k)` → Bool lookup. `map.length(m)` = `array.len $keys`.
- `map.into(source, target)` → guard **target only** (an iterated *source* is
  legal), then merge source entries in source order (overwrite-in-place or
  append); `map.into(m, m)` self-merge is a guarded no-op. Returns target.

**(c) Literals.** `expression kind "map"`: for each k/v pair left to right
(key expr, then value expr), then a put — duplicate dynamic keys behave as put
(overwrite in place). `{}` builds an empty map from the contextually checked
type; the emitter picks the repr pair from `x.typ` (mirrors `mapTypes`).

**(d) Iteration.** `for v in m` yields **values**; `for k, v in m` binds
**key first, value second**. Emission copies the `for-collection` pattern:
snapshot `$keys`/`$vals` arrays into locals, `$active += 1` (+ a `watCleanup`
entry so an early word `return` restores it, as M1 does for lists), loop over
the snapshot, restore. Guarded mutation inside fails 10 (`runtime error:
collection is being iterated`, unchanged). Guard fires *before* the operation —
even a no-op `map.remove` of a missing key during iteration fails (test
`TestMapMutationGuardCoversNoOpsAndAliases`).

**(e) Traversal-vs-mutation parity.** The guard is a counter (nested traversal
of one map stacks); it must release on every exit of the guarding helper.
Prelude helpers restore `$active` before *each* return path (the interpreter
uses `defer`) — early exits in `any`/`all`/`find` are the bug farms. Compiled
traps kill the run, so trap paths need no cleanup (M1 precedent).

## 4. Callbacks via typed funcrefs

**(a) Function-value representation.** One uniform closure struct plus
typed funcrefs per callback signature:

```wat
(type $zhex_closure (struct (field $fn (ref func)) (field $env (ref null $zhex_arr_ref))))
(type $zhex_clo_<p1>_<p2>_r_<r> (func (param (ref $zhex_closure)) (param <p1> <p2>) (result <r>)))
```

`<p>/<r>` are M1 element reprs (i64/i32/eqref-level ref) — repr-level, so e.g.
`(String)->Int` and `(List<Int>)->Int` share `$zhex_clo_ref_r_i64`; arity 1–2
covers every M2 callback position. `$env` holds *all* target-word parameters in
order with holes as null; captured values are boxed with the M1 Result-payload
boxing (Int → `$zhex_some_i64`, Bool → `ref.i31`, else the ref itself). The
function type's repr is `(ref null $zhex_closure)` everywhere (bindings, word
params/results, ref-spec collections, `castFromEqref` gains the function-type
case).

**(b) Creation (emitter).** At a partial site `f(a, _, c)`: evaluate the
supplied args left to right (holes skipped — eval order), box into an
`array.new_fixed $zhex_arr_ref` of target-arity, and pair with a **site-local
thunk**:

```wat
(func $zcthunk<N> (param $clo (ref $zhex_closure)) (param $a0 <p1>) (result <r>)
  ;; unbox env slots per the static target signature, fill the hole slots
  ;; from $a0..., then (call $zw_f) — a direct call; casts to concrete
  ;; struct types for ref params happen here, where types are known.
```

Thunks are plain functions (no `$zhex_enter` — they are not words; the target
word's own enter/leave keeps depth parity, and builtins never counted). Builtin
partials (`text.trim(_)`, `text.contains(_, "Al")`, `text.length(_)` in
sort-by) emit thunks that call the prelude helper directly; only builtins the
emitter supports may be partialized, others fence via the base fence. The
closure value: `ref.func $zcthunk<N>`, env array, `struct.new $zhex_closure`.

**(c) Invocation (prelude).** Each callback word has per-signature prelude
helpers, e.g. `$zhex_list_map_<in>_<out>(list, clo)`, `$zhex_list_fold_<t>_
<acc>(list, acc, clo)`, `$zhex_list_sort_by_<t>_i64 / ..._ref(list, clo)`. The
invoke sequence is always:

```
<args...>      ;; loop item (and accumulator for fold)
local.get $clo / struct.get $zhex_closure $fn / ref.cast (ref $zhex_clo_...)
call_ref $zhex_clo_...
```

Amendment: the original sketch put the funcref on the stack first; the
stack-valid order is args first, then the funcref — `call_ref` consumes the
arguments, then the funcref. E's emitted order (see `closureInvoke` in
`wat.go`) is the valid one.

**(d) Per-word shapes** (source: `host.go`; all set `$active` on the source and
restore on every exit; all build fresh outer lists with shared elements):
- `list.map(values, f)` — call f per item in order, collect results.
- `list.filter` / `list.select` — identical semantics for lists (true keeps the
  element *unmodified*); one shared prelude body is fine.
- `list.fold(values, init, step)` — `step(acc, item)` per item; returns the
  final accumulator (or `init` unchanged when empty).
- `list.find(values, pred)` — first match → `$zhex_some_<spec>` of the element;
  none → `$zhex_none` (variant numbers match M1's Option: None 0 / Some 1).
- `list.any` / `list.all` — short-circuit exactly as the interpreter: any stops
  at the first true, all at the first false; empty → false / true.
- `list.sort(values)` — no callback; stable ascending, Int by `i64` compare,
  String by code-point compare (`$zhex_str_lt`, new: element-wise `i32.lt_u`,
  which equals Go's UTF-8 byte order). Fresh outer list, source unchanged.
- `list.sort-by(values, key)` — two phases: evaluate `key(item)` once per
  element in source order into a parallel keys array, then stable-sort the
  items by keys (Int or String). Stability via insertion sort that shifts only
  on strictly-greater keys (O(n²) is acceptable; parity beats speed).
- `array.select(values, pred)` — rank-2 only, else `$zhex_fail_msg(15, ...)`
  with the rank formatted by `$zhex_int_to_string`; each row is passed as a
  **fresh List<T> copy** of the row's width elements (an append inside the
  predicate must not corrupt the source); kept rows append their items; result
  is a fresh rank-2 array shaped `[kept, width]`; source `$active` guarded.

A closure may be `let`-bound and passed on, and fully-applied direct calls
`f(x)` of a bound closure are emitted (cast + `call_ref`, closure as arg 0).
Function-typed word parameters/results work through the uniform repr.

## 5. Core deltas: `while` (and the already-shipped string+Int)

- **`while cond` + body**: statement of type `Unit` (never a binding RHS — the
  checker enforces). Condition evaluated *before* each pass; no break/continue;
  **no iteration limit and no depth consumption** (the interpreter's `while`
  touches neither `r.depth` nor any counter; an infinite loop hangs by design).
  Emission: eval cond into a local `c`, then
  `block $exit / loop $loop / local.get c; i32.eqz; br_if $exit / body
  statements (statementNeutral each — eval discards the block value) / re-eval
  cond into c / br $loop`. A `return` inside the body `br $zhex_ret`s out
  (crossing loop labels is legal; M1's cleanup list already restores traversal
  counters). Runtime errors inside the body trap as usual (§2c/M1 §6).
- **String-context Int exception / `print` Int**: already emitted by M1
  post-merge (`wat.go` binary `+` under `x.typ == stringType`; `printCall`
  `intType`). M2 ships fixtures only; one guard: two Int operands still add
  numerically and `Bool` never converts — checker-guaranteed, emitter just
  follows node types.

## 6. Fence inventory after M2

Everything below still fails emission with code `not-available-in-build`,
message `%s is not available in this build`. The M1 hint text hardcodes
"milestone-1 subset"; M2 rewords it once, milestone-neutral, so it stops
churning per milestone: `this construct is not compiled by zhex compile yet;
run it with zhex run instead` (fence tests assert the new string). Fenced:

- `file.*` — all of: `read open read-line close write stream list exists size
  free-space join base extension directory-of make-folder delete delete-folder
  move` (M3; the pure path words stay fenced with the family).
- `read-line`; `for line in file.stream`.
- `buffer.*` — `new add-line line-count line set-line to-text` (M3).
- `json.decode json.encode json.parse`; `xml.decode` (M3).
- `env.get env.load` (M3).
- All temporal: `date.* time.* duration.* instant.* timezone.* zoned.*`
  (`duration.` arrives M3 per D3).
- `console.arg console.args-count console.exit console.fail`.
- All `error.*` — `new wrap report set-reporter has-cause cause code message
  operation file line column detail` (the caught §2 err value stays opaque).
- Checked collection forms (`try`/`catch` on `list.append`, `array.new`,
  `array.resize`, `file.exists`) — unchanged from M1 (they need recoverable
  Results from prelude helpers, a different mechanism than the fail trap).
- Word parameter defaults; record field defaults; generic choices.
- Partials that leave holes when *called* (re-partial application,
  partial-of-partial) and function values as bare flow steps — the §4 closure
  covers creation, passing, and full application only.
- `print` of anything but Int/Bool/String/Unit (unchanged).

## 7. Differential fixtures (`internal/wasm/testdata/`, one per family)

1. `text.zhex` — every §1 word; multibyte (`héllo`, `😀`, `e`+U+0301 kept as
   two code points), NBSP trim, slice clamps (`-2,99`, `start>=end`),
   `index-of` −1/empty needle, split edges (leading/repeated/trailing empties,
   empty separator on multibyte, empty text × both separators, multibyte
   separator), join round-trip, literal replace, line/word counts, starts/ends
   (empty prefix/suffix), upper/lower (`éà`/`ÉÀ`), reverse with combining mark.
2. `text-to-int.zhex` — success cases (whitespace-padded, `+7`, leading zeros,
   MinInt64/MaxInt64) and a caught fallback; a separate `text-to-int-fail.zhex`
   ends with an unhandled `try` → exit 1, stderr exactly
   `runtime error: "x" is not a signed 64-bit decimal integer`.
3. `maps.zhex` — literal + put-overwrite (position kept) + remove middle +
   re-put (moves to end) + `into` (incl. iterated-source merge), single/dual
   iteration in insertion order, Option lookup match, aliasing; a separate
   `maps-iterate-fail.zhex` mutates during `for` → exit 1, stderr
   `runtime error: collection is being iterated`.
4. `callbacks.zhex` — map/filter/select/fold/find (Some and None)/any/all with
   user words and builtin partials (`text.trim(_)`, `text.contains(_, "Al")`),
   fresh-outer-list witness (append to source and to result afterwards), shared
   elements, `let`-bound closure reused twice, empty inputs.
5. `sort.zhex` — Int and String sorts (code-point witness: `Éclair éclair
   żebra 😀`), stable `sort-by` ties over records, key-run-once-in-source-order
   witness (noisy key that prints), `sort-by` with `text.length(_)`.
6. `while.zhex` — counter, initially-false, nested while, while-in-word,
   collatz (computed condition) as the ticket's witness.
7. `string-int.zhex` — `"a"+1`, `1+"a"`, `1+2+"a"`, `1+"a"+2`, `"a"+1+"b"`,
   negative Int render, direct `print 5` (regression for the pre-shipped §5).
8. `array-select.zhex` — rank-2 row keep, rectangle preserved, row-copy witness
   (append to the row inside the predicate); `array-select-rank.zhex` uses a
   rank-3 array → exit 1, stderr `runtime error: array.select requires a
   rank-2 array, found rank 3`.
9. Fence tests (emitter unit, no tools): one per §6 family asserting
   `not-available-in-build` and the new hint wording.

## 8. Builder split (3-way, no shared files)

- **R — runtime prelude M2**: new `internal/lang/watprelude_m2.go` (const
  `watPreludeM2`, emitted by `wat.go`'s `emit()` immediately after `watPrelude`
  — E owns that one-line concat) and new `internal/lang/watnames_m2.go` (fail
  codes 12–15, `$zhex_err`, map/closure/text/sort helper and type names, per
  this doc). Optional `watprelude_m2_test.go` sanity (assemble a hand module,
  skip-safe).
- **E — emitter + fences**: edits `internal/lang/wat.go` and
  `internal/lang/wat_test.go` (while, map literal/index/iteration, text/map/
  callback builtin cases, closure creation and full application, catch-payload
  change, fence-hint rewording, emit-order concat).
- **T — fixtures + gate wiring**: `internal/wasm/compile_test.go` and
  `internal/wasm/testdata/*` (new fixtures, `run.mjs` fail-table extension for
  codes 12–15 with staged-message decoding).
- Shared surfaces are exactly: the names R defines in `watnames_m2.go` (E
  emits, R defines — checked in first), this doc's §2c code table (R and T),
  and the fixture list (T). Nobody edits another builder's files.

## 9. Default choices taken (not pinned by #95/DESIGN.md)

- Fixed 64 buckets, no resize; splitmix64/FNV-1a hashes — traversal order is
  hash-independent, so any deterministic hash is correct.
- Insertion sort for stable sorts (O(n²) accepted; parity over speed).
- Closure = uniform `{fn: (ref func), env: eqref array}` + per-site thunks with
  a leading env parameter; boxing reuses M1's Result-payload helpers.
- `text.length` inline; every other text word a prelude call.
- `quote` is ASCII-exact and passes non-ASCII through literally (§2b shortcut).
- Case tables generated offline from Go's `unicode.CaseRanges` and pasted as
  delta-encoded constants; the generator is not part of the build.
  (Amended 2026-11-15: the constants drifted across Go/Unicode versions — the
  tables now generate at process start from the running toolchain's `unicode`
  package, keeping compiled upper/lower identical to the interpreter built
  from the same Go.)
- `catch` binds the err payload (not the Result struct) — M1 behavior amended.
- Fail codes 12–15; staged UTF-8 at offset 0 with an exported `zhex_msg_len()`.
- Map guard is a counter checked before the operation, restored on every
  helper exit path; no size limit on maps.
- Fence hint reworded milestone-neutral once (§6).
- Direct fully-applied calls of bound closures are in scope; re-partial
  application and function-valued flow steps stay fenced.
- No new lang catalog codes (M2 adds no compile-time diagnostics of its own
  beyond the existing `not-available-in-build`).
- `$zhex_str_lt` added beside `$zhex_str_eq` (code-point `<`, = Go byte order).
- wasm-tools 1.263 quirk: `i64.shr_u` validates only with an i64 shift count
  (an i32 count is rejected), so prelude shifts carry i64 counts.
