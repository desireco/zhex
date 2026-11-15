# Zhex → WebAssembly, Milestone 4: delta contract

Binding inputs: `internal/wasm/DESIGN.md` (M1), `DESIGN-M2.md`, `DESIGN-M3.md` — all hold
except where amended here — `docs/wasm-plan.md` (D1–D7, Part 6 M4 row), ticket #102, and the
interpreter at `internal/lang` (`xml_codec.go`, `xml_test.go`, `host.go`, `eval.go`,
`collections.go`). Interpreter semantics were pinned from that code and from probing Go's
`encoding/xml` on this toolchain, not from memory; where wording differs, the code wins.
Acceptance stays: differential fixtures byte-identical stdout and exit status vs `zhex run`;
stderr follows the M1 convention (`runtime error: ` replaces the interpreter's position).
Only the delta from M3 is specified.

## Surprises (read first)

1. **`wasm-m1` lags main.** M1/M2 were merged to main and then reverted (`a9bc511`); main has
   since gained the doctest and test-runner tickets (#93/#94/#97), so main's corpus is 41
   `.zhex` files vs the branch's 39, and `math.zhex` now carries `## >` doctests. The
   inventory below classifies **main's** corpus (the arc-end merge lands it). **First M4 step:
   merge main into `wasm-m1`** so the gate walks the real corpus. Main's `xml_codec.go` diff
   (`dc66921`) is a pure field-index refactor — semantics identical, verified by diff.
2. **`xml-config` needs one more unfence than the ticket names**: `service.endpoints[0] try`
   is a *checked list index read* (`checkedIndex` in the emitter, fenced since M1). Without
   it, acceptance criterion 2 cannot pass. §4 adds the checked **list** read (arrays stay
   fenced — no corpus driver).
3. **The trio's flow-step member is nearly free.** `flowStatement` fences `step.binding`, but
   the machinery behind it (`flow-value` cursor arg + `closureInvoke`) already handles it;
   the shared blocker is Unit-result closures (the closure functype grid has no void-result
   row). One grid extension unfences two trio members at once.
4. **Re-partial needs no new representation.** A `$zhex_closure` is already an eqref that
   `boxEnv` passes through unboxed — the target closure rides the env array as one extra
   slot and the thunk `call_ref`s it. No new types beyond a static cast target.
5. **No new fail codes.** xml failures are Results: `catch` sees the opaque `$zhex_err`
   payload, unhandled top level reuses M3's code-16 staged path; the checked list read is a
   Result too. Codes 1–18 stand; **19+ stay reserved**; `run.mjs` needs no changes.
6. **Go xml errors carry line numbers** (`XML syntax error on line N: …`), pinned by probe:
   N = 1 + count of raw `\n` before the error position (entity-decoded newlines never
   count); mismatched-close reports at the closing tag; EOF reports at end of input.
7. **NBSP is in the trim set** — `text.to-int("\u00a0x")` fails on `x` but quotes the
   *untrimmed* input, so the NBSP fixture must prefix garbage (`"\u00a0x"`), not pad a valid
   number. And the fix is asymmetric in file messages: the wrapper `<q>` is `%q`-quoted, the
   inner `PathError` path renders raw (M3 §1b) — only the wrapper sees `\u00a0`.
8. **The corpus run matrix already exists** — `tools/site-data/main.go exampleSpecs` pins
   per-example stdin (`"Ada\n"`, `"48\n18\n"`, …) and args. The gate copies it; no new
   invention. In-subset examples take no args (the arg-taking ones are all out-of-subset).

## 1. Corpus inventory (the factual backbone)

41 files under `examples/` (main's tree). Classification verified by running the M3 compiler
over each file; "blockers" lists every fenced construct present, not just the first hit.
After xml + the trio + the checked list read: **xml-config flips in; nothing else moves** —
the trio unblocks no corpus file alone (partials stays out on word defaults + `error.*`).

**In-subset today — 23** (compile, run, byte-compare): `collatz`, `composition`,
`edit-distance`, `factorial`, `fibonacci`, `fizzbuzz`, `gcd`, `hello`, `json-tree`, `life`,
`line-filter`, `maps`, `modules/main`, `modules/math`, `modules/md-report/main`,
`modules/md-report/md`, `palindrome`, `paths`, `records`, `recovery`, `score-report`,
`sorting`, `times-table`. Constructs exercised across the set: words + recursion +
short-form calls, `let`/`var`/`if`/`elif`/`match`, `while`, `for range` and collection
iteration (dual bindings), lists/arrays/maps with literals, aliases, and legal guarded
paths, all in-subset `text.*`, `list.*` callbacks with user words and builtin partials,
`sort`/`sort-by`, `json.parse`/`encode` + Json-tree walks, pure path words, `buffer.*`,
`read-line`/`text.to-int` with `try`/`catch`, flows (computed seeds, call-shaped steps
with args), `use` modules with private words, records positional + named, `Option` match.

**In-subset after M4 — +1**: `xml-config` (blockers today: `xml.decode`, checked list read;
both claimed here).

**Out-of-subset — 17, with every blocker and its disposition:**

| file | blockers | disposition |
|---|---|---|
| `calls` | word parameter defaults | fenced (lazy default eval — not in #102) |
| `choices` | generic choice `Maybe<T>` | fenced (generics out of v1) |
| `collections` | checked `list.append` form, `error.code`, `array.shape`, `array.resize` | fenced (checked collection forms + `error.*` + two array words — not in #102) |
| `env-config` | `console.arg` | fenced (`console.*` — interpreter-side argv has no host seam in-subset) |
| `exit-status` | `error.code`, `console.exit` | fenced (`error.*`, `console.*`) |
| `file-work` | `console.arg`, `file.free-space` | fenced (argv; free-space is a statfs host call, D4 leaves it interpreter-only) |
| `file-write` | `error.code` | fenced (`error.*`) |
| `filter` | `console.args-count`, `console.fail`, `console.arg` | fenced (`console.*`) |
| `json-config` | record field defaults | fenced (constructor-filled defaults — not in #102) |
| `partials` | word parameter defaults, Unit-result closures*, function value as flow step*, `error.code` | *claimed here; stays out on word defaults + `error.*` |
| `temporal` | `timezone.load`, `zoned.*`, `date.new`, `time.new` | fenced forever in v1 (D3 — tz database stays out of artifacts) |
| `text-stats` | `console.arg`, `console.args-count` | fenced (`console.*`) |
| `word-defaults` | word parameter defaults (incl. print-side-effect defaults) | fenced |
| `xml-config` | `xml.decode`, checked list read | **claimed here — flips in** |
| `modules/html-page/html.zhex` | `error.new` (in `html.link`) | fenced (`error.*`) |
| `modules/html-page/main.zhex` | `error.new` (transitive via module) | fenced |
| `modules/math.test.zhex`, `modules/test.zhex` | `console.fail` (in `test.zhex`) | fenced (`console.*`) |

The gate (§6) asserts each row's membership and blocker set; the fenced diagnostic must
name a construct in the file's blocker list (emission order picks the first — not pinned).

## 2. `xml.decode` design

Pure computation over the string repr — no host imports. Semantics pinned from
`xml_codec.go` + `xml_test.go` (main); error messages byte-pinned. New prelude
`$zhex_xml_decode(raw (ref $zhex_string), defaults (ref null eq), schema (ref null $zjson_schema)) (ref $zhex_result)`; the emitter case mirrors `json.decode` (defaults boxed via
`boxEnv`, schema via the M3 `$zjson_schema<N>` builder — reused verbatim, one builder per
call site; works as a flow step exactly as json.decode does).

**(a) Element tree** (new rec group in `watprelude_m4.go`): `$zxml_elem {name str, attrs
(ref null arr_ref) of $zxml_attr{name, value}, text (mut str), children (mut (ref null
arr_ref)) of elem refs}`; text accumulates by `$zhex_str_concat` (O(n²) accepted; parity
beats speed). Parser = one code-point-domain scanner (line counting is raw-`\n`; identical
in either domain) mirroring Go's decoder for every fixture-reachable behavior:

- Token order per token kind, checks in the interpreter's order: **element name** prefix
  check (`:` → fail `namespace-qualified name <q local>` , code xml-field, reason
  prefixed, path = stack path *excluding* the element) → multiple-root (`<a/><b/>` after
  root → `multiple root elements`, xml-syntax) → element count > 1,000,000
  (`document exceeds element limit`, xml-range) → depth `len(stack) >= 256`
  (`XML nesting limit exceeded`, xml-depth) → **attrs** left-to-right: `xmlns=…` → fail
  with the *element's* local name (Go sets the default ns, element fails), `xmlns:p` or any
  prefixed attr → fail with the local part. Namespace rule is the simple first-failure
  equivalence: namespaces are always fatal in-subset, so the declaring/using element always
  fails before scope matters. ProcInst after root or inside an element → `processing
  instruction outside the prolog`; any `<!…` that is not `<!--` or `<![CDATA[` → `DTD and
  entity declarations are rejected`; comments skipped; CDATA is raw CharData
  (no entity decoding); CharData outside the root ignored; no root at EOF → `no root
  element`; trailing text after root ignored.
- Entities in text and attribute values: `lt gt amp apos quot`, `&#N;`, `&#xH;`; XML
  legality check (legal: `\t \n \r`, 0x20–0xD7FF, 0xE000–0xFFFD, 0x10000–0x10FFFF — else
  `illegal character code U+XXXX`, uppercase hex); undefined name → `invalid character
  entity &<name>;`; missing `;` → `invalid character entity &<text> (no semicolon)`;
  out-of-range numeric → `invalid character entity &#x110000;` (literal as written).
  (2026-11-15 amendment: the legality check's strict-letter wording is retired for the
  surrogate band — numeric refs `&#xD800;`–`&#xDFFF;` do not fail; both engines decode
  them to U+FFFD identically, probe-verified in the interpreter and the compiled skins.)
- Malformed XML wraps Go's phrasing: `invalid XML: XML syntax error on line N: <detail>`
  with the detail table (probed on this toolchain): `unexpected EOF` (unterminated
  tag/element, at input end), `element <open> closed by </close>` (position = closing tag),
  `unexpected end element </x>` (stray close), `invalid XML name: <text>`, `unquoted or
  missing attribute value in element`, `attribute name without = in element`. shortcut:
  fixtures stick to these shapes; extend the table on divergence (M3 json precedent).
- Name grammar shortcut: ASCII `[A-Za-z_][A-Za-z0-9_.-]*` (fixtures are ASCII-named;
  upgrade the ranges on divergence). Attribute values verbatim (entities decoded, no XML
  whitespace normalization — Go doesn't either; fixtures avoid newlines in values).
- UTF-8 validity of `raw`: pre-checked as `XML is not valid UTF-8` (xml-syntax) — module
  strings are valid by construction, so the check is a surrogate scan; unreachable,
  pinned for completeness.

**(b) Schema-directed decode** — the interpreter shares `jsonSchema` + the clone memo, and
so does the prelude: record/list/map/option/int/bool/string nodes, `$make`/`$get` thunks,
and the linear-scan `ref.eq` identity memo for defaults deep-clone (M3 §6 machinery,
unchanged). Pinned rules: attributes and child elements map onto record fields **verbatim
by name**; unknown attr/element → `unknown attribute <q>` / `unknown element <q>`
(xml-field, reason unknown, path `…/<name>`); collision both directions → `attribute <q>
collides with a child element` / `element <q> collides with an attribute` (reason
collision); repeated non-list element → `duplicate element <q>` (reason duplicate);
non-whitespace text among children → `mixed content is not supported` (xml-type, actual
text); repeated map child → `duplicate map key` (xml-field, reason duplicate, path
`…/<name>`); missing field without default → `missing required field <q>` (reason
missing); list children decode at `path/i`; map children by tag name; element limit
`list exceeds element limit` (xml-range) when children > 1,000,000 at a list/map node.
Scalars: attrs present → `expected <Type>` (actual attribute); children present →
`expected <Type>` (actual element); else text. Int: trim (M2 §1 set) then
ParseInt-base-10 with the to-int overflow split — ErrRange → `expected an integer-form
signed 64-bit number` (xml-range), other → `expected Int` (xml-type). Bool: trimmed text
exactly `true`/`false`, else `expected Bool`. String: raw untrimmed text. Option: element
or attribute present → `Option.Some` of the inner decode (with the default's `Some`
payload passed down as the inner default, exactly `decodeXMLText`'s `inner` fallback);
absent → clone the default. Schema kinds beyond the six stay a checker error before emission
(`xml.decode` unsupported-schema is checked pre-run in the interpreter; same in compiled —
the checker rejects before WAT exists).

**(c) Failure values.** Every xml failure is `err` with `$zhex_err{family 5 (Error),
message "<message> at \"<path>\""}` — the interpreter's `xmlError` renders
`fmt.Sprintf("%s at %q", f.message, f.path)`; root path is `""`. code/details
(offset/line/column/reason/expected/actual) are interpreter-envelope metadata, unobservable
while `error.*` is fenced — not compiled. Unhandled top level → M3 code-16 staging, e.g.
`runtime error: unknown element "ghost" at "/ghost"`.

## 3. The fenced trio — all three compile

**(a) Unit-result closures — compile.** The only blocker is the M2 closure functype grid
having no void result (`watCloType` maps Unit to the `ref` spec; a void `call_ref` cast
would be a stack bug, hence the fence). Design: a `v` result marker — six new grid rows
`$zhex_clo_<p>_r_v` / `$zhex_clo_<p>_<p>_r_v` (`(func (param (ref $zhex_closure)) (param …)`
with no result) declared in `watprelude_m4.go` (arity 0 impossible — a partial needs a
hole). `partialCreation` and `closureInvoke` drop their `fnResult == unitType` fences;
`closureInvoke` casts to the `_r_v` type and skips the final `castFromEqref`;
`appendThunk` already omits the result section. (2026-11-15 amendment: the "no such builtin
position" claim is false — the interpreter's `list.map`/`list.fold` do accept Unit-result
callbacks, and the emitter correctly fences those two sites (`wat.go`, "callbacks with
Unit results"); the sentence stands only for the other callback helpers.) Void closures as non-final flow steps
make the cursor Unit — the existing `cur=""` path handles it. Justification: nothing about
Unit is unrepresentable — wasm void is the natural repr of a Zhex word that produces no
value; `print _` (partials.zhex) is the flagship witness and the fence was a naming-gap
artifact, not a semantic gap.

**(b) Re-partial application — compile.** At a partial site over a *binding*
(`f(a, _)` where `f` holds a closure): evaluate the binding first (it is the callee), then
supplied args left-to-right, holes skipped; env array = target arity + 1 — slots `0..n-1`
as today (captured boxed, holes null), slot `n` = the target `$zhex_closure` ref itself
(an eqref; no boxing). The site thunk takes `(closure-self, one param per hole)` in the
*partial's* signature, then per target position: hole → its param (`castFromEqref` to the
target param type), captured → `envGet(i)`; finally `array.get` slot `n` →
`struct.get $fn` → `ref.cast` to the **target's** `$zhex_clo_…` type (statically known
from the binding's checked function type) → `call_ref`. Void targets use (a)'s `_r_v` row.
Partial-of-partial composes: the re-partial's target is again just a closure binding.
Depth parity: thunks never `$zhex_enter`; the target word's own enter/leave governs —
unchanged. Justification: the closure struct already round-trips env arrays as eqrefs; the
re-partial is the same thunk pattern with an indirect instead of direct target — no new
machinery, and the alternative (re-fence) would leave `f(_, 2)` on a let-bound `f` legally
checked but uncompilable, which is exactly the class of hole M4 exists to close.

**(c) Function values as flow steps — compile.** Delete `flowStatement`'s
`step.binding != nil` fence: `e.call(step)` then routes to `closureInvoke` with `args[0]`
already the `flow-value` cursor local; extra step args (`step.args[1:]`) and fallible
steps (`try` → `emitResultUnwrap`) ride the existing paths. A step whose *extra* args
contain holes routes to `partialCreation` — legal once (b) lands; the trio lands as one
unit. Justification: the interpreter's flow loop is `callable = env[step.binding]` plus
`[cursor, args[1:]]` — the compiled seam already mirrors it; the fence predates closures
existing at all (M1) and simply was never revisited when M2 shipped them.

## 4. Checked list index read

`indexExpr` with `x.checkedIndex` on a `List` (checker type `Result<T, Error>`, family 5):
new prelude `$zhex_list_get_checked_<spec>(list, i) (ref $zhex_result)` — in bounds → tag 0
+ M1 boxing; out of bounds → tag 1 + `$zhex_err{5, "list index is out of range"}`
(interpreter `listRead`: code list-range, operation `list.index`, same message; details
index/length stay envelope-only). Checked **array** reads and all checked writes/collection
forms stay fenced (no corpus driver; one line in §1's table records them).

## 5. NBSP quoting fix

`$zhex_quote` / `$zhex_cp_esc_len` in `watprelude_m2.go` (the M2 §2b shortcut comment lives
there; fixed in place, M2's post-hoc amendment precedent): match Go `strconv.Quote` exactly.
ASCII arm unchanged (printables literal; `\"  \\` + `\a\b\f\n\r\t\v`; `\xHH` lowercase for
the rest incl. 0x7F). New non-ASCII arm: literal iff `unicode.IsPrint(cp)`, else `\uXXXX`
(cp < 0x10000) / `\UXXXXXXXX`, lowercase hex. The printable-range table generates at
process start from the running toolchain's `unicode` package (the case-table amendment,
M2 §9) — delta-encoded, binary-searched; surrogates cannot occur (lexer-valid code points).
Fixtures: `text.to-int("\u00a0x") try` unhandled → `runtime error: "\u00a0x" is not a
signed 64-bit decimal integer`; and a file witness (Surprises 7): wrapper quotes, inner
PathError path raw.

## 6. Corpus gate

`internal/wasm/corpus_test.go` + a `just corpus` target (`go test ./internal/wasm/ -run
'TestCorpusGate' -v`); the same test also runs under plain `go test ./...` (skip-safe via
`requireTool`, so tool-less CI stays green — CI installs the tools, 0b4f128).

- The test **walks `../../examples` (recursive, `*.zhex`)** and fails on any file missing
  from the pinned table — zero silently-skipped files; a new corpus file on main breaks the
  gate until classified.
- Table row per file: `{in bool, stdin string, blockers []string}`. stdin copied from
  `tools/site-data exampleSpecs` (`hello "Ada\n"`, `gcd`/`modules/main "48\n18\n"`,
  `factorial "5\n"`, `fibonacci "10\n"`, `fizzbuzz "15\n"`, `times-table "7\n"`,
  `palindrome "racecar\n"`; others empty); in-subset entries take no args. A one-line sync
  note in both files: the manifest is the human-facing matrix, the gate's the enforced one.
- In-subset: compile (sibling resolver for `use` — small re-implementation of the CLI's
  reader, modules resolve against the entry file's directory in the real tree), run the
  interpreter (`lang.Run`, stdin fed, args none) and the module (browser skin via
  `run.mjs --stdin`), compare stdout bytes + exit status. No in-subset example touches the
  filesystem, so no seeds/`--root` are needed. WASI pass under `wasmtime` where installed
  (skip-safe), same assertion.
- Out-of-subset: compile must fail; report code `not-available-in-build`; the named
  construct must appear in the row's `blockers` (exact spelling not pinned — emission
  order chooses).
- Budget: 41 compiles (wasm-tools parse+validate ≈ 150 ms each) + ~24 node runs ≈ well
  under a minute; go-test's per-test timeout is the ceiling. No sharding now; if the corpus
  doubles, shard by top-level directory with `-run 'TestCorpusGate/<dir>'` (note only).

## 7. Fuzz differential

`internal/wasm/fuzz_test.go`, `TestFuzzDifferential`: a deterministic generator of
in-subset programs, **50 per run**, each compared interpreter-vs-compiled (browser skin),
stdout bytes + exit status. Expected output is *derived* — the interpreter is the oracle;
there is no expected-output file.

- **Determinism**: `rand.New(rand.NewSource(20261115 + int64(i)))` per program — never the
  `math/rand` global functions (auto-seeded per process since Go 1.20 — NOT deterministic);
  no `time`, no `os` entropy, and the generator itself iterates slices only (no Go map
  iteration order). Same seeds → same 50 programs on every machine and every run.
- **Grammar** (in-subset constructs only): 1–8 explicitly annotated words (`Int`/`String`/
  `Bool` params, `->` results; recursion only fib/gcd-shaped with args ≤ 30); `let`/`var`,
  `if`/`elif`/`else`, `match` over a generated choice + `Option`; `while` only as a
  counter-decrement shape (`var c = <n ≤ 20>; while c > 0 … c = c - 1` — the interpreter's
  while has no iteration limit, so the generator must guarantee termination itself);
  `for n in range(…)` with bounds ≤ 100; lists (`new`/`append`/`length`/index reads),
  literals and aliases; maps (literal, `put`, lookup `match`, single+dual iteration);
  the full in-subset `text.*`; `sort`/`sort-by` with partial keys; flows (word steps,
  call-shaped steps with args, computed seeds); closures — word and builtin partials,
  full application of let-bound closures, and after the trio: Unit-result closures,
  re-partials, closure-valued flow steps. Arithmetic unconstrained: division by zero and
  overflow probe the trap paths (both engines exit 1 with equal stdout — the comparison
  covers it); int literals mostly |v| < 2^62 with occasional Min/MaxInt64.
- **Bounds**: ≤ 60 statements, string literals ≤ 12 code points over an alphabet of ASCII,
  multibyte (`é`, `😀`), NBSP, and escapes (`\"`, `\n`); program source ≤ 8 KB.
- **Excluded**: every still-fenced construct (M3 §8's list plus the M4 leftovers —
  `console.*`, `error.*`, checked array/collection forms, word/record defaults, generic
  choices, temporal, json/xml; xml stays fixture-pinned, not fuzzed) and, for
  determinism, the host-dependent words the compiled target *does* support (`read-line`,
  `file.*`, `env.*`) — the generator emits pure programs only.
- Runs in the normal suite; ~50 × 250 ms ≈ 12 s — CI-viable. The count is one constant;
  raising it is a one-line change with a matching budget note.

## 8. Differential fixtures (testdata/, browser skin; WASI mirrors where relevant)

1. `xml.zhex` — attributes + entities (`&amp; &#65;`), list of records, `Map<String, Int>`
   children, `Option` absent/present, scalar roots (`<n>7</n>`, empty elements), defaults
   deep-clone aliasing witness, `xml.decode` as an inferred flow step; `xml-fail.zhex` →
   `runtime error: unknown element "ghost" at "/ghost"`; `xml-syntax.zhex` →
   `runtime error: invalid XML: XML syntax error on line 1: element <body> closed by </doc> at ""`;
   plus undefined-entity and DTD-rejection lines.
2. `trio.zhex` — `print _` typed `(Int) -> Unit` (statement call, void flow step), re-partial
   chains (word target: hole-then-captured and captured-then-hole; builtin target), closure
   flow steps with extra args, fallible closure invoked with `try`.
3. `nbsp.zhex` — the §5 witnesses (to-int quote; file-read wrapper-vs-inner asymmetry).
4. Fence unit tests (emitter, no tools): `array.shape`, `error.message`, `console.exit`,
   checked `array` read, word defaults → `not-available-in-build` + M2's milestone-neutral
   hint (regression).

## 9. Truth-surface checklist (docs builder)

1. `docs/wasm-plan.md` — new final section **"Part 9 — Artifact sizes"**: per-family
   artifact sizes measured at M4 (smallest `hello`, mid `score-report`, largest
   `modules/md-report/main`, plus `xml-config`), raw and `gzip -9n` bytes, with the
   toolchain record (wasm-tools 1.263, Node 22, wasmtime 50) and the one-liner method
   (`wasm-tools parse` → `wc -c` / `gzip -9n | wc -c`).
2. `docs/getting-started.md` — new "## Compile to WebAssembly" section: canonical
   `zhex compile examples/hello.zhex`, both targets (`--target=browser` default, run under
   Node/browser; `--target=wasi` + `wasmtime run --dir .::/ hello.wasm`), the wasm-tools
   requirement stated openly with one-line installs — `cargo install wasm-tools`,
   `pacman -S wasm-tools`, `brew install wasm-tools`, or a release binary from the
   BytecodeAlliance releases page — and the coded missing-tool diagnostic (D6).
3. `site/src/content/docs/compiling.md` — NEW site page (frontmatter section `Tools`),
   the full compile story: command, targets, wasm-tools installs per platform, missing-tool
   diagnostic, out-of-subset `not-available-in-build` fence with the run-instead hint.
4. `AGENTS.md` — two edits: (a) canonical commands gain `zhex compile` (the `cmd/zhex`
   repo-map line and the Verifying-work bullet: wasm-affecting changes also run
   `just corpus`); (b) the ticket-workflow step-6 Done-rule correction — current wording
   "Commit and push, let the deploy go green, then move the card to Done" is wrong per the
   owner's ruling; replace with the green-build rule, quoting it: #100 closed "**Done —
   build green on `wasm-m1`.**" and #7 records "landed on the branch and closed (#100,
   green-build rule)" — i.e. a green build on the branch moves the card to Done and closes
   the issue; pushing/merging never gates Done (ships when the arc merges).
5. `ROADMAP.md` — the Future WASM bullet (line 56) moves to Done **at the arc-end merge,
   not in M4**; its merge-time text is prepared: "WebAssembly compiler: `zhex compile`
   ships with browser and WASI targets, byte-identical to the interpreter across the
   examples corpus (#7)." The generator's Done/Next/Future shape must stay valid.
6. `site/llms.txt` — the compile command enters the agent workflow recipe: `zhex compile
   file.zhex [-o out.wasm] [--target=browser|wasi]`, wasm-tools requirement + installs,
   and the `not-available-in-build` fence concept (run fenced constructs with `zhex run`).
7. Sync check only (no edit expected): `internal/lang/codes.go`/`catalog.md` — M4 adds no
   lang codes; `not-available-in-build` is cataloged since M1; `missing-tool` stays
   CLI-side.

## 10. Builder split (3-way, disjoint, per the owner's claim on #102)

- **B1 — runtime + emitter**: `internal/lang/watnames_m4.go` (**checked in first** — B2
  compiles against it): the `_r_v` grid rows, `$zxml_*` type/helper names, checked-read
  helper names, no new fail codes. Then `internal/lang/watprelude_m4.go` (xml parse +
  decode, clo grid rows, checked list read), the `$zhex_quote` NBSP fix in place in
  `watprelude_m2.go`, and the `wat.go` + `wat_test.go` deltas (xml.decode case, checked
  read case, the three fence removals + re-partial thunk emission + void-closure naming).
  Preload sanity tests in `watprelude_m4_test.go` (skip-safe).
- **B2 — corpus + fuzz gate**: `internal/wasm/corpus_test.go`, `internal/wasm/fuzz_test.go`,
  `testdata/xml*.zhex`, `testdata/trio.zhex`, `testdata/nbsp.zhex`, the `justfile` corpus
  target. `run.mjs` unchanged (Surprises 5).
- **B3 — docs truth**: everything in §9 (wasm-plan sizes section, getting-started,
  `compiling.md`, AGENTS.md's two edits, ROADMAP merge-time text recorded on #7, llms.txt).
- Shared surfaces are exactly: `watnames_m4.go` names (B1 defines first), §2c's message
  strings (B1 prelude, B2 fixtures), the §1 corpus table + §6 run matrix (B2), and §9's
  wording (B3). Nobody edits another builder's files; the only pre-existing files touched
  are `wat.go`/`wat_test.go`/`watprelude_m2.go` (B1), `justfile` (B2), and the docs/AGENTS/
  ROADMAP/llms.txt set (B3). Order: B1 lands first, B2 gates green on it, B3 runs parallel
  once B1's CLI surface is stable (it already is — no `cmd/zhex` changes in M4).

## 11. Default choices taken (not pinned by #102 or the M1–M3 contracts)

- No new fail codes or `$zhex_err` families: xml and the checked read are family 5 through
  the existing err struct; unhandled failures stage via code 16. Codes 19+ reserved.
- Checked **list** read only; checked array reads/writes and the other checked collection
  forms stay fenced (§1 records the blockers; no corpus or fuzz driver).
- Namespace handling = first-failure simple rule (§2a) — observationally equivalent because
  every namespace event is fatal before scope can propagate.
- xml names ASCII-grammar shortcut; attribute values verbatim (no XML whitespace
  normalization, matching Go); CDATA raw; element text accumulates by concatenation.
- xml decode reuses the M3 json schema builders and clone memo verbatim, one builder per
  call site, no caching (M3 precedent).
- Void closures via a `v` result marker in the clo grid (6 rows, arity 1–2); re-partial
  target rides env slot `arity`, callee evaluated before args; thunks stay depth-free.
- Fuzz: fixed seed base 20261115, 50 programs, per-program `rand.New(rand.NewSource(base+i))`;
  interpreter is the oracle; stdout + exit status only (stderr carries positions in the
  interpreter per the M1 convention and is not compared).
- Corpus gate: browser skin for every in-subset file, WASI pass skip-safe beside it; stdin
  from the site manifest; no FS seeding (no in-subset example touches disk).
- Artifact sizes measured once at M4 and recorded in-plan (Part 9), not regenerated in CI.
- Not claimed (named, one line each, per the lazy-senior rule): `console.*`, `error.*`,
  word/record defaults, generic choices, checked array/collection forms, `array.shape`,
  `array.resize`, `file.free-space`, temporal beyond `duration.`, `list.map`/`list.fold`
  with Unit-result callbacks (the interpreter accepts them; the void-callback grid has no
  collection-callback rows yet, §3a amendment) — all stay fenced with
  `not-available-in-build` and honest blocker rows in §1.
