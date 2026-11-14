# Standard library research and proposed plan

Research for [#30](https://github.com/desireco/zhex/issues/30), 2026-11-14. The owner approved creating these stories for one-by-one review on 2026-11-14; this is not a specification of new language behavior. The current library inventory below is implemented; every later signature marked proposed is unimplemented. Implementation stories #57–64 were filed in Inbox, then explicitly triaged by the owner to Todo; review and select each before implementation.

## Recommendation

Grow through a line-filter/report program first, then reusable collection transformations. Keep the current small core and data-first flows. Add literal text operations before patterns, file output before networking, and concrete integer helpers before a numeric tower. Do not delay useful concrete words for generic user words or a package manager.

The strongest lessons are Go's explicit namespaces and host boundaries, Python's precise copy/sort contracts, Ruby's collection vocabulary, Gleam's data-first composition, Elm's small core with separate capabilities, and Haskell's separation of pure transformations from effects. These are design inputs, not a request to reproduce six libraries.

## Current inventory and evidence

The baseline is commit `b4b6f59`. [`hostWords` in check.go](../internal/lang/check.go) registers **84 names**. Constructors, operators, bracket access/assignment, and REPL commands are additional syntax or session facilities, not additional registered host words. The table is a complete name inventory; the linked site references specify signatures and edge cases.

| Owner | Shipped words | Reference and executable evidence |
| --- | --- | --- |
| Bare | `print`, `range`, `read-line`, `ok`, `err` | [Console](../site/src/content/docs/console.md), [Results](../site/src/content/docs/results.md); `io_test.go`, `result_words_test.go`, `features_test.go` |
| `int` | `to-string` | [Text](../site/src/content/docs/text.md); `text_test.go`, `examples_test.go` |
| `text` | `reverse`, `to-int`, `length`, `line-count`, `word-count`, `upper`, `lower`, `trim`, `contains`, `index-of`, `slice`, `replace` | [Text](../site/src/content/docs/text.md); `text_test.go`, `examples_test.go` |
| `list` | `new`, `length`, `append` | [Collections](../site/src/content/docs/collections.md); `rich_values_test.go`, `plain_collection_test.go` |
| `array` | `new`, `resize`, `shape`, `rank`, `length` | [Collections](../site/src/content/docs/collections.md); `rich_values_test.go`, `plain_collection_test.go` |
| `map` | `length`, `contains`, `remove`, `put`, `into` | [Maps](../site/src/content/docs/maps.md); `maps_test.go` |
| `json` | `decode`, `encode` | [JSON](../site/src/content/docs/json.md); `json_test.go` |
| `buffer` | `new`, `add-line`, `line-count`, `line`, `set-line`, `to-text` | [Buffers](../site/src/content/docs/buffers.md); `buffer_test.go` |
| `file` | `read-all`, `read-lines`, `open`, `read-line`, `close`, `lines` | [Files](../site/src/content/docs/files.md); `file_test.go` |
| `console` | `arg`, `args-count`, `exit`, `fail` | [Console](../site/src/content/docs/console.md); `args_test.go`, CLI `main_test.go`, `reporting_test.go` |
| `error` | `set-reporter`, `report`, `new`, `wrap`, `has-cause`, `cause`, `code`, `message`, `operation`, `file`, `line`, `column`, `detail` | [Results](../site/src/content/docs/results.md); `result_words_test.go`, `reporting_test.go` |
| `date` | `new`, `parse`, `to-string`, `add-days` | [Temporal](../site/src/content/docs/temporal.md); `temporal_test.go`, `civil_time_test.go` |
| `time` | `new`, `parse`, `to-string` | Same temporal reference/tests |
| `duration` | `new`, `parse`, `to-string` | Same temporal reference/tests |
| `instant` | `now`, `parse`, `to-string`, `add-duration` | Same temporal reference/tests |
| `timezone` | `load`, `name` | Same temporal reference/tests; this research adds direct `name` identity/type coverage |
| `zoned` | `new`, `to-string`, `instant`, `in-zone`, `add-days`, `add-duration` | Same temporal reference/tests |

The [test catalog](tests.md) maps the broader behavior contract to tests. An inventory audit found all 84 names in the site references, with direct test-source mentions for all except `timezone.name`; that gap is covered in this change. Literal occurrence is an audit aid, not proof of exhaustive edge-case coverage. No existing operation is being redesigned here.

Important current constraints:

- `String` operations are specified in Unicode code points. Source literals, file reads and `read-line` validate UTF-8, but the current `console.arg` path returns host argument bytes without validation; arbitrary Unix argv therefore exposes an existing boundary gap. This research does not change runtime behavior. Length, slice and index positions count code points, not bytes or graphemes; `index-of` uses `-1` for absence. Preserve these contracts while deciding the argv validation policy separately. There is no normalization, locale-aware casing, or byte type.
- `Int` is checked signed 64-bit. There is no Float, Decimal, BigInt, generic user word, or generic record.
- Lists, arrays, maps, buffers, and files have shared identity. List/array iteration rejects mutations of the active container. A new outer list does not imply a deep copy of nested values.
- Routine collection operations are plain with selected opt-in checked forms. Parsing and external input use Results. A Result cannot be deferred, stored inside a collection, or passed as a user-word parameter.
- Explicit partial function values exist, but generic higher-order built-ins still need checker/runtime support and a callback-effect contract. Omitted parameter annotations infer one concrete type; they are not polymorphism.
- `use` loads sibling files, not installed standard packages. Generic source libraries and package resolution are separate future work; #30 decides content, not a new import mechanism.

## Comparison matrix

“Separate” means outside that language's core/base package, not necessarily third-party. Sources below identify the ownership boundaries. Coverage is descriptive, not a ranking by number of APIs.

| Area | Go | Ruby | Python | Gleam | Elm | Haskell |
| --- | --- | --- | --- | --- | --- | --- |
| Text / Unicode | `strings`, `unicode`; string indexing is bytes | String bytes/chars/graphemes | Unicode `str`, separate `bytes` | String APIs include grapheme operations | Core String, separate Char | Base String/Char; efficient Text separate |
| Numbers | Fixed-width numbers, `math`, separate `math/big` package in stdlib | Integer/Float and numeric classes | int/float; decimal/fractions modules | Int/Float modules | Core Int/Float operations | Base numeric classes, Int/Integer |
| Collections | slices/maps, generic helpers | Array/Hash/Enumerable blocks | list/dict/set, iterators and itertools | Typed generic List/Dict/Set and combinators | Core List/Array/Dict/Set | Base lists and folds; containers separate |
| Files / paths / I/O | `io`, `os`, `path/filepath` | IO/File/Dir, Pathname library | io/os/pathlib | Small portable core; platform packages for OS work | No general OS filesystem in core; browser capabilities separate | Base System.IO; directory/filepath separate |
| Dates / time | `time` | Time, Date library | datetime/time/zoneinfo | Separate packages | Separate official time package | Separate time package |
| Parsing / interchange | strconv, encoding/json/csv | Conversions; JSON/CSV libraries | Conversions; json/csv | Core parsing helpers; JSON separate | Core string conversion; official json separate | Read/Show in base; JSON ecosystem |
| Network / process | net/http, os/exec | Libraries plus process facilities | urllib/http/subprocess | Runtime-specific and ecosystem packages | Official HTTP via commands; no core subprocess | process/network packages; HTTP ecosystem |
| Testing | testing package and go test | Separate test tooling/libraries | unittest/doctest | Separate gleeunit package | Separate elm-explorations/test | Separate HUnit/QuickCheck ecosystem |

### Source-backed lessons

**Go.** The [standard package inventory](https://pkg.go.dev/std) supplies a broad host-facing toolkit. [strings](https://pkg.go.dev/strings) offers literal split/join and prefix/suffix operations, but its byte indices must not leak into Zhex. [slices](https://pkg.go.dev/slices) shows that generic collection helpers can have explicit constraints. [filepath.Clean](https://pkg.go.dev/path/filepath#Clean) is lexical cleanup, not symlink resolution or sandbox containment. [os.LookupEnv](https://pkg.go.dev/os#LookupEnv) distinguishes a missing name from an empty value. Prefer Go as an implementation substrate, with independent Zhex semantics and diagnostics.

**Ruby.** Its [documentation index](https://docs.ruby-lang.org/en/master/) distinguishes core classes and libraries; these rolling development docs are not a pinned release promise. [String](https://docs.ruby-lang.org/en/master/String.html) distinguishes bytes, characters, and grapheme clusters. [Enumerable](https://docs.ruby-lang.org/en/master/Enumerable.html) is a useful vocabulary source for map/filter/find/fold, but Zhex should not inherit aliases, implicit protocol dispatch, or dynamic coercions. Standard-library gem packaging is not equivalent to core language support.

**Python.** The [library inventory](https://docs.python.org/3/library/) shows a large built-in/stdlib boundary. [Text and binary types](https://docs.python.org/3/library/stdtypes.html) remain separate. The [sorting guide](https://docs.python.org/3/howto/sorting.html) specifies stable sorting, keys evaluated once, and the distinction between copied `sorted` results and mutation. [PurePath versus Path](https://docs.python.org/3/library/pathlib.html) is a useful separation of lexical operations and effects. Prefer these explicit contracts over duplicating Python's breadth or exceptions.

**Gleam.** [List](https://hexdocs.pm/gleam_stdlib/gleam/list.html) uses list-first generic transformations, close to Zhex's flow order; `try_map` stops at the first error. `filter_map` intentionally discards failed results, which would undermine Zhex's visible handling rule if copied casually. Its [String](https://hexdocs.pm/gleam_stdlib/gleam/string.html) grapheme behavior is a useful contrast, not grounds to silently change Zhex code-point indexing.

**Elm.** [Core List](https://raw.githubusercontent.com/elm/core/1.0.5/src/List.elm) demonstrates a compact typed vocabulary and optional lookup. Its callback-first/list-last order suits Elm pipelines; Zhex should retain data-first order instead. Elm's separation of core from capability packages is the useful architectural lesson, rather than trying to import its application runtime or effect system.

**Haskell.** [Base](https://hackage.haskell.org/package/base) offers pure list operations and explicit I/O; packages such as text and containers are distinct from base. GHC's [list implementation/documentation](https://raw.githubusercontent.com/ghc/ghc/master/libraries/ghc-internal/src/GHC/Internal/Data/OldList.hs) describes stable sorting and once-per-element keys for `sortOn`. Borrow those determinism guarantees, not laziness, type classes, partial `head` behavior, or pervasive currying.

### Package boundaries behind the matrix

Gleam's [JSON](https://hexdocs.pm/gleam_json/) and [HTTP](https://hexdocs.pm/gleam_http/) packages are separate from gleam_stdlib; [gleam_erlang](https://hexdocs.pm/gleam_erlang/) exposes target-specific process/port facilities. [Simplifile](https://hexdocs.pm/simplifile/) and [Birl](https://hexdocs.pm/birl/) are community filesystem and time packages. [Gleeunit](https://hexdocs.pm/gleeunit/) is a separate testing dependency.

Elm's [core manifest](https://raw.githubusercontent.com/elm/core/1.0.5/elm.json) defines the core surface. Official [JSON](https://raw.githubusercontent.com/elm/json/1.1.3/elm.json), [HTTP](https://raw.githubusercontent.com/elm/http/2.0.0/elm.json), [time](https://raw.githubusercontent.com/elm/time/1.0.0/elm.json), and [file](https://raw.githubusercontent.com/elm/file/1.0.5/elm.json) are separate packages; file means browser selection/download, not arbitrary OS access. [elm-explorations/test](https://raw.githubusercontent.com/elm-explorations/test/master/elm.json) is also separate. Core [Process](https://raw.githubusercontent.com/elm/core/1.0.5/src/Process.elm) means cooperative task scheduling, not shell processes. The [effects guide](https://guide.elm-lang.org/effects/) explains the runtime boundary.

Haskell's [base manifest](https://raw.githubusercontent.com/ghc/ghc/master/libraries/base/base.cabal.in) is distinct from the [GHC boot package roster](https://raw.githubusercontent.com/ghc/ghc/master/packages), which includes text, containers, time, directory, filepath, and process. [Aeson](https://raw.githubusercontent.com/haskell/aeson/master/aeson.cabal), [http-client](https://raw.githubusercontent.com/snoyberg/http-client/master/http-client/http-client.cabal), and [QuickCheck](https://raw.githubusercontent.com/nick8325/quickcheck/master/QuickCheck.cabal) are ecosystem packages. Do not label all of Hackage as the standard library.

The first-class Result combinators in [Gleam](https://hexdocs.pm/gleam_stdlib/gleam/result.html) and [Elm](https://raw.githubusercontent.com/elm/core/1.0.5/src/Result.elm) depend on storing and passing Results as ordinary values. Zhex deliberately cannot copy that API without changing its handling contract.

## Ownership and implementation rules

1. A dot identifies the owner or domain: text operations live under `text`, list transformations under `list`, and external filesystem work under `file`. Keep compound verbs hyphenated. Do not add aliases merely because another language has them.
2. Keep the existing bare fundamental operations. New domain utilities should be qualified. Conversions retain explicit names such as `int.to-string`; no implicit stringification is proposed.
3. Put the flowing data first, then configuration/callbacks. Existing APIs remain compatible. An output helper may therefore use `file.write-all(contents, path)`, with the content first; document this deliberately alongside path-first reads.
4. Use host built-ins for UTF-8 primitives, resource effects, and collection operations needing checker polymorphism. Use concrete Zhex words for policies/algorithms expressible clearly today: integer min/max helpers, line-filter orchestration, report rendering, Life rules, edit-distance recurrence.
5. A reusable source module is an option today only when its signatures are concrete and its sibling import works. An installed standard-module search path is not part of this plan. Future optional libraries need a separate distribution design.
6. Ordinary deterministic transformations return ordinary values. Expected absence uses `Option` in new lookup APIs; preserve the existing `text.index-of` sentinel. Invalid programmer inputs diagnose. Fallible I/O/parsing returns one explicit Result family. Do not wrap every pure call in `try`.
7. New collection outputs use fresh outer storage; nested mutable values remain shared unless a separately specified deep-copy facility is introduced. Callbacks run in source order, with traversal mutation guards that release on every exit.

## Proposed phases and draft stories

These are reviewable Todo stories, not implemented features. Each story must first add failing behavioral tests, then implementation, a registered executable example or an updated existing example, contract/reference/llms updates, `just test`, and `go vet ./...`. Diagnostics must have stable codes; wrong types must fail before effects. Keep all existing examples and error behavior compatible. New names require checking user-name and module-reservation collisions.

### Phase 1 — one useful text/report workflow

**S1. Literal text decomposition and reconstruction.** Proposed host signatures:

```text
text.split(value: String, separator: String) -> List<String>
text.join(values: List<String>, separator: String) -> String
text.starts-with(value: String, prefix: String) -> Bool
text.ends-with(value: String, suffix: String) -> Bool
```

Demand: a proposed `line-filter` program selects records by literal prefix/suffix and joins selected fields for a report; this is the next application recommended by [measurements](measurements.md). Split on a nonempty literal separator preserves leading, repeated, and trailing empty fields; no implicit trim. Proposed empty-separator behavior: split into code points, with empty input producing an empty list; nonempty separator with empty input produces one empty field. Empty prefix/suffix matches. Join of an empty list is empty text. Returned lists have fresh outer storage. Enforce the existing one-million-element list limit before allocating/materializing an unbounded split output; ordinary limit failure diagnoses, with an opt-in checked form if integrated into the existing collection recovery mechanism. Specify and test invalid-UTF-8 input behavior, including the existing argv boundary gap, before assuming every host-supplied string is valid. Tests must cover UTF-8, combining marks, emoji, empty values, repeated/trailing separators, limits, and data-first flow use. No regex, grapheme normalization, or whitespace mode. `text.words`/`text.lines` and prefix removal wait for demonstrated repeated code.

**S2. Persist a text report.** Proposed host signature:

```text
file.write-all(contents: String, path: String) -> Result<Unit, FileError>
```

Demand: save the proposed line-filter report or a Life generation snapshot; current programs can print but cannot write a file. This story depends on the report workload, not new generics. Specify create-or-truncate/overwrite, no implicit parent creation, UTF-8 contents with explicit handling of invalid host-supplied argument strings, and resource closure. Errors can leave a partial/truncated file; do not promise atomic replacement. Test existing/missing paths, empty contents, write/close failures and short writes through suitable host seams, and failure propagation/reporting. Preserve error-family rules when composing with console arguments; demonstrate explicit wrapping if families need unification. A separate atomic-save API requires a later portability contract.

**S3. Reduce the edit-distance recurrence.** Start with concrete source words `min(left: Int, right: Int) -> Int` in the algorithm or a sibling module. Demand: replace the manual three-way minimum in `examples/edit-distance.zhex` while preserving exact results. This can ship independently of S1/S2. S1's proposed empty-separator split also permits precomputing code-point lists once instead of repeatedly slicing whole strings in the nested loop; benchmark that change before claiming a speedup. Only promote `int.min`/`int.max` into the core after a second workload demonstrates reuse; a two-line helper alone does not justify more built-ins. If promoted, include MinInt64/MaxInt64 and equal-value cases. Do not add `abs` without specifying MinInt64 overflow, or Float/math APIs without new type design. Life already has sufficient rectangular arrays; no new grid type is needed.

### Phase 2 — reusable collection composition

**S4. Pure-result list transformations.** Proposed checker-polymorphic host signatures (the `T`, `U`, `A` notation describes built-ins, not supported user generic syntax):

```text
list.map(values: List<T>, transform: (T) -> U) -> List<U>
list.filter(values: List<T>, predicate: (T) -> Bool) -> List<T>
list.fold(values: List<T>, initial: A, step: (A, T) -> A) -> A
```

Demand: map and filter line-filter records and fold report totals without bespoke loops. Depends on S1 for the illustrative workload and on extending callable type constraints, not on generic user words. Accept existing explicit partials (`text.trim(_)`), not new lambda syntax. Reject callbacks whose return type is Result in this story. “Pure-result” does not mean effect-free: ordinary callbacks may print or mutate other values. Evaluate arguments once; invoke callbacks once per element left-to-right. Hold the source container's traversal guard, release it on all exits, and preserve side effects already performed before a runtime failure. New map/filter outer lists are fresh, nested values shared; folding may return the explicit accumulator object. Empty input gives empty output or the unchanged initial accumulator. Test nested aliasing, source mutation through aliases, callbacks with effects, inference failures/ambiguous empty collections, limits, and function values imported from modules.

Proposed usage, intentionally not yet runnable:

```text
flow text.split(" Ada , Grace ", ",")
    list.map(text.trim(_))
    text.join(" / ")
    print
```

**S5. Short-circuit search and predicates.** After S4's callback contract:

```text
list.find(values: List<T>, predicate: (T) -> Bool) -> Option<T>
list.any(values: List<T>, predicate: (T) -> Bool) -> Bool
list.all(values: List<T>, predicate: (T) -> Bool) -> Bool
```

Demand: find the first malformed report record and check whether any/all rows match a rule. Return `None` for no match; empty any is false and empty all is true. Stop immediately when determined, including observable callback effects. Same alias/guard/type rules as S4. Keep this a separate story so ordinary transformation work does not expand into every Enumerable method.

**S6. Deterministic copied sorting.** After S4:

```text
list.sort(values: List<Int>) -> List<Int>
list.sort(values: List<String>) -> List<String>
list.sort-by(values: List<T>, key: (T) -> Int) -> List<T>
list.sort-by(values: List<T>, key: (T) -> String) -> List<T>
```

These are proposed constrained overloads, not new String relational operators. Demand: rank report records and print a deterministic ordered summary. Stable ascending order; String order is lexicographic Unicode code-point order, without locale collation or normalization. Evaluate keys exactly once in original order before sorting; reject Result keys initially. Fresh outer list, shared elements, unchanged source order. Test ties with record identities, empty input, negative numbers, Unicode, callback counts/effects and alias mutation rejection. No arbitrary comparator, reverse flag, or map reordering until demanded.

**S7. Fallible traversal, separately.** Only after S4 and an explicit callback-effect design:

```text
list.try-map(values: List<T>, transform: (T) -> Result<U, E>) -> Result<List<U>, E>
```

Demand: parse all numeric report fields without silently dropping invalid input. Stop at the first failure, preserve its error family/origin, never invoke later callbacks, and do not expose a partial result list. Effects from earlier callbacks remain; no transaction promise. Infer exactly one callback family, reject ambiguity and mixed-family misuse, and integrate top-level propagation, reporters, trace behavior, partials, and source guards. No `List<Result<...>>`, `filter-map` that discards errors, or Result-valued user parameters. A fallible fold can follow only if a concrete workload requires it.

### Phase 3 — optional capabilities after evidence

**S8. Paths and configuration for batch reports.** Stage behind a multi-file report example. Candidate host operations `path.join(base: String, child: String) -> String`, `path.base(path: String) -> String`, `path.extension(path: String) -> String`, and `env.get(name: String) -> Option<String>`. Define OS-native lexical semantics (not URLs), absolute-child behavior, empty/root/trailing-separator cases, invalid environment names/encoding, and missing versus empty values. No existence check hidden in cleanup, no containment guarantee, no process launching. Reserve namespaces only when shipping and assess compatibility with existing sibling modules.

Directory listing/metadata belongs in a separate story with deterministic ordering and explicit I/O errors. CSV needs a real tabular interchange example and quoting/newline contract; split is not a CSV parser. Temporal extraction/comparison should follow a scheduling/report workload rather than another broad date API pass. A user-facing testing library should start from a failing real Zhex test suite and specify runner discovery, assertions, failure locations and exit status; keep the existing Go executable specification meanwhile.

## Deferred and excluded scope

- **HTTP client/server:** optional capability work, not Phase 1. [Go Client.Do](https://pkg.go.dev/net/http#Client.Do) illustrates why status codes are response data while transport failures are errors. Define response records, closure, timeout, redirects, body caps, headers and text/binary representation before introducing a getter. Server lifecycle is another story.
- **Process execution:** optional host library. [Go exec](https://pkg.go.dev/os/exec) and [Python subprocess](https://docs.python.org/3/library/subprocess.html) support explicit argument vectors. Specify launch failure versus nonzero-exit data, timeout, capture limits and encoding; no implicit shell-string API.
- **Regex:** defer until literal operations fail a real parsing task. [Go regexp](https://pkg.go.dev/regexp) provides input-linear matching, but compiled-pattern values, limits and code-point match positions need a Zhex contract.
- **Randomness, hashing, binary encodings:** define a byte representation and capability boundaries first where needed. [Python secrets](https://docs.python.org/3/library/secrets.html) separates security tokens from simulation randomness. Do not offer an ambiguous universal random function.
- **Generic source libraries, installed modules, package management:** separate language/distribution work. Monomorphic source helpers and generic built-in checking are sufficient for these early phases.
- **No implicit coercion, automatic currying, lazy collection semantics, arbitrary precision, generic equality, Result-erasing convenience, or giant compatibility layer.** None is needed to validate the first report workflow.

## Decision and completion boundary

Recommended first batch: S1 and S2, with S3 independently improving the existing algorithm example. Then S4, followed by evidence-driven S5/S6 and the deliberately separate S7. S8 and other optional capabilities stay proposals.

The owner approved story creation and one-by-one review. The stories were first filed in Inbox with no status, then explicitly moved to Todo at the owner's request; unresolved edge contracts stay explicit for review. Research and story creation do not implement these APIs or change the current reference.

| Order | Story | Dependency |
| --- | --- | --- |
| S1 | [#57 — Literal text split/join and prefix/suffix](https://github.com/desireco/zhex/issues/57) | Establish line-filter workload |
| S2 | [#58 — Persist a text report](https://github.com/desireco/zhex/issues/58) | #57 report workload; Life snapshot is another validation option |
| S3 | [#59 — Simplify edit-distance minimum](https://github.com/desireco/zhex/issues/59) | Independent |
| S4 | [#60 — List map/filter/fold](https://github.com/desireco/zhex/issues/60) | #57 workload and existing callable support |
| S5 | [#61 — Short-circuit find/any/all](https://github.com/desireco/zhex/issues/61) | #60 callback contract |
| S6 | [#62 — Stable copied sort/sort-by](https://github.com/desireco/zhex/issues/62) | #60 callback contract |
| S7 | [#63 — Explicit fallible try-map](https://github.com/desireco/zhex/issues/63) | #60 plus callback-effect design |
| S8 | [#64 — Paths/environment for batch reports](https://github.com/desireco/zhex/issues/64) | #57/#58 and a multi-file workload; later phase |

No story is In Progress merely because it appears in this order. #30 tracks completion of research and backlog preparation; each story follows the normal claim, implementation, verification, and close-last workflow when selected.

## Additional dogfooding exploration

After approving the eight library stories, the owner also approved [#65 — a Zhex-written REPL front end](https://github.com/desireco/zhex/issues/65) as a bounded investigation/prototype. Zhex would own prompt, buffer, and dispatch policy over the existing Go Session engine; lexing, checking, evaluation, and terminal machinery stay host-owned. The story must examine a minimal typed session bridge, indefinite input without recursive stack growth, EOF/cancellation, and shared program/source input. Transcript parity comes before any production cutover, with terminal parity a separate later gate. This is not a promise to rewrite the interpreter in Zhex. It was filed in Inbox and then moved to Todo at the owner's request; it can inform library gaps without depending on every earlier story.
