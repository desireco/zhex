# Standard library research and proposed plan

Research for [#30](https://github.com/desireco/zhex/issues/30), 2026-11-14. The owner approved creating these stories for one-by-one review on 2026-11-14; this is not a specification of new language behavior. The current library inventory below is implemented; every later signature marked proposed is unimplemented. Implementation stories #57–64 were filed in Inbox, then explicitly triaged by the owner to Todo; #57–62 have since shipped, #63 was declined and removed by the owner (2026-11-14), and #64 awaits one-by-one review.

## Recommendation

Grow through a line-filter/report program first, then reusable collection transformations. Keep the current small core and data-first flows. Add literal text operations before patterns, file output before networking, and concrete integer helpers before a numeric tower. Do not delay useful concrete words for generic user words or a package manager.

The strongest lessons are Go's explicit namespaces and host boundaries, Python's precise copy/sort contracts, Ruby's collection vocabulary, Gleam's data-first composition, Elm's small core with separate capabilities, and Haskell's separation of pure transformations from effects. These are design inputs, not a request to reproduce six libraries.

## Current inventory and evidence

The baseline is commit `b4b6f59`; stories S1 (#57), S2 (#58), S4 (#60), S5 (#61), and S6 (#62) have since shipped four text words, one file word, six list callback words, and two list sorting words; #68 renamed the read side to `file.read` and `file.stream` while retiring `file.read-lines`; S9 (#73) shipped four file inspect words, S11 (#74) four lexical path words, and S10 (#75) four destructive file words; #76 shipped `json.parse` over the host-predeclared `Json` tree type; #70 added the two select-by-predicate words; #79 shipped `xml.decode` on the stdlib encoding/xml tokenizer. [`hostWords` in check.go](../internal/lang/check.go) registers **112 names**. Constructors, operators, bracket access/assignment, and REPL commands are additional syntax or session facilities, not additional registered host words. The table is a complete name inventory; the linked site references specify signatures and edge cases.

| Owner | Shipped words | Reference and executable evidence |
| --- | --- | --- |
| Bare | `print`, `range`, `read-line`, `ok`, `err` | [Console](../site/src/content/docs/console.md), [Results](../site/src/content/docs/results.md); `io_test.go`, `result_words_test.go`, `features_test.go` |
| `int` | `to-string` | [Text](../site/src/content/docs/text.md); `text_test.go`, `examples_test.go` |
| `text` | `reverse`, `to-int`, `length`, `line-count`, `word-count`, `upper`, `lower`, `trim`, `contains`, `index-of`, `slice`, `replace`, `split`, `join`, `starts-with`, `ends-with` | [Text](../site/src/content/docs/text.md); `text_test.go`, `examples_test.go` |
| `list` | `new`, `length`, `append`, `map`, `filter`, `fold`, `find`, `any`, `all`, `sort`, `sort-by` | [Collections](../site/src/content/docs/collections.md); `rich_values_test.go`, `plain_collection_test.go`, `list_words_test.go`, `sort_test.go` |
| `array` | `new`, `resize`, `shape`, `rank`, `length` | [Collections](../site/src/content/docs/collections.md); `rich_values_test.go`, `plain_collection_test.go` |
| `map` | `length`, `contains`, `remove`, `put`, `into` | [Maps](../site/src/content/docs/maps.md); `maps_test.go` |
| `json` | `decode`, `encode`, `parse` | [JSON](../site/src/content/docs/json.md); `json_test.go` and `json_tree_test.go` |
| `buffer` | `new`, `add-line`, `line-count`, `line`, `set-line`, `to-text` | [Buffers](../site/src/content/docs/buffers.md); `buffer_test.go` |
| `file` | `read`, `open`, `read-line`, `close`, `write`, `stream`, `list`, `exists`, `size`, `free-space`, `join`, `base`, `extension`, `directory-of` | [Files](../site/src/content/docs/files.md); `file_test.go`, `file_inspect_test.go`, `path_test.go` |
| `console` | `arg`, `args-count`, `exit`, `fail` | [Console](../site/src/content/docs/console.md); `args_test.go`, CLI `main_test.go`, `reporting_test.go` |
| `error` | `set-reporter`, `report`, `new`, `wrap`, `has-cause`, `cause`, `code`, `message`, `operation`, `file`, `line`, `column`, `detail` | [Results](../site/src/content/docs/results.md); `result_words_test.go`, `reporting_test.go` |
| `date` | `new`, `parse`, `to-string`, `add-days` | [Temporal](../site/src/content/docs/temporal.md); `temporal_test.go`, `civil_time_test.go` |
| `time` | `new`, `parse`, `to-string` | Same temporal reference/tests |
| `duration` | `new`, `parse`, `to-string` | Same temporal reference/tests |
| `instant` | `now`, `parse`, `to-string`, `add-duration` | Same temporal reference/tests |
| `timezone` | `load`, `name` | Same temporal reference/tests; this research adds direct `name` identity/type coverage |
| `zoned` | `new`, `to-string`, `instant`, `in-zone`, `add-days`, `add-duration` | Same temporal reference/tests |

The [test catalog](tests.md) maps the broader behavior contract to tests. An inventory audit found all 111 names in the site references, with direct test-source mentions for all except `timezone.name`; that gap is covered in this change. Literal occurrence is an audit aid, not proof of exhaustive edge-case coverage. No existing operation is being redesigned here.

Important current constraints:

- `String` operations are specified in Unicode code points. Source literals, file reads and `read-line` validate UTF-8, but the current `console.arg` path returns host argument bytes without validation; arbitrary Unix argv therefore exposes an existing boundary gap. This research does not change runtime behavior. Length, slice and index positions count code points, not bytes or graphemes; `index-of` uses `-1` for absence. Preserve these contracts while deciding the argv validation policy separately. There is no normalization, locale-aware casing, or byte type.
- `Int` is checked signed 64-bit. There is no Float, Decimal, BigInt, generic user word, or generic record.
- Lists, arrays, maps, buffers, and files have shared identity. List/array iteration rejects mutations of the active container. A new outer list does not imply a deep copy of nested values.
- Routine collection operations are plain with selected opt-in checked forms. Parsing and external input use Results. A Result cannot be deferred, stored inside a collection, or passed as a user-word parameter.
- Explicit partial function values exist and serve as callbacks to the shipped `list.` transformation and search words; fallible traversal (S7, #63) was declined and removed by the owner on 2026-11-14, and a select-shaped word is drafted separately as [#70](https://github.com/desireco/zhex/issues/70). Omitted parameter annotations infer one concrete type; they are not polymorphism.
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
3. Put the flowing data first, then configuration/callbacks. Existing APIs remain compatible. An output helper may therefore use `file.write(contents, path)`, with the content first; this deliberate asymmetry with path-first reads is documented alongside the shipped word.
4. Use host built-ins for UTF-8 primitives, resource effects, and collection operations needing checker polymorphism. Use concrete Zhex words for policies/algorithms expressible clearly today: integer min/max helpers, line-filter orchestration, report rendering, Life rules, edit-distance recurrence.
5. A reusable source module is an option today only when its signatures are concrete and its sibling import works. An installed standard-module search path is not part of this plan. Future optional libraries need a separate distribution design.
6. Ordinary deterministic transformations return ordinary values. Expected absence uses `Option` in new lookup APIs; preserve the existing `text.index-of` sentinel. Invalid programmer inputs diagnose. Fallible I/O/parsing returns one explicit Result family. Do not wrap every pure call in `try`.
7. New collection outputs use fresh outer storage; nested mutable values remain shared unless a separately specified deep-copy facility is introduced. Callbacks run in source order, with traversal mutation guards that release on every exit.

## Proposed phases and draft stories

These are reviewable stories; S1 (#57), S2 (#58), S3 (#59), S4 (#60), S5 (#61), and S6 (#62) have shipped, S9 (#73), S11 (#74), and S10 (#75) have shipped from the folder-operations research below, #76 (the JSON heterogeneous tree) has shipped from the document research, #70 (predicate select) has shipped from the owner interview, S7 (#63) was declined and removed by the owner (2026-11-14), S8 (#64) remains a Todo proposal, and the document-libraries research below added candidates S12/S14/S15 (the XML typed decode shipped in its S12 slot as [#79](https://github.com/desireco/zhex/issues/79), S14 shipped as the pure-Zhex `md` module [#77](https://github.com/desireco/zhex/issues/77), and S15 remains a candidate awaiting owner triage); the unshipped candidates are not implemented features. Each story must first add failing behavioral tests, then implementation, a registered executable example or an updated existing example, contract/reference/llms updates, `just test`, and `go vet ./...`; the full surface list for a new word, signature change, or rename/removal is the [vocabulary-change checklist](vocabulary-checklist.md), which each story follows instead of re-deriving the surfaces. Diagnostics must have stable codes; wrong types must fail before effects. Keep all existing examples and error behavior compatible. New names require checking user-name and module-reservation collisions.

### Phase 1 — one useful text/report workflow

**S1. Literal text decomposition and reconstruction.** Shipped as [#57](https://github.com/desireco/zhex/issues/57); host signatures:

```text
text.split(value: String, separator: String) -> List<String>
text.join(values: List<String>, separator: String) -> String
text.starts-with(value: String, prefix: String) -> Bool
text.ends-with(value: String, suffix: String) -> Bool
```

Demand: a proposed `line-filter` program selects records by literal prefix/suffix and joins selected fields for a report; this is the next application recommended by [measurements](measurements.md). Split on a nonempty literal separator preserves leading, repeated, and trailing empty fields; no implicit trim. Empty-separator behavior: split into code points, with empty input producing an empty list; nonempty separator with empty input produces one empty field. Empty prefix/suffix matches. Join of an empty list is empty text. Returned lists have fresh outer storage. Enforce the existing one-million-element list limit before allocating/materializing an unbounded split output; ordinary limit failure diagnoses, with an opt-in checked form if integrated into the existing collection recovery mechanism. Specify and test invalid-UTF-8 input behavior, including the existing argv boundary gap, before assuming every host-supplied string is valid. Tests must cover UTF-8, combining marks, emoji, empty values, repeated/trailing separators, limits, and data-first flow use. No regex, grapheme normalization, or whitespace mode. `text.words`/`text.lines` and prefix removal wait for demonstrated repeated code.

Shipped notes: split enforces the one-million-element limit before materializing output, reusing the `collection-limit` code with the message `text.split exceeds element limit`; no new diagnostic codes were added. Invalid-UTF-8 input behaves like the existing code-point text words (no validation), and the argv gap stays open as a pre-existing boundary. The opt-in checked split recovery is deferred with the collection recovery mechanism. The `line-filter` example demonstrating the shipped words is registered; the full file-reading `line-filter` application recommended by [measurements](measurements.md) remains open.

**S2. Persist a text report.** Shipped as [#58](https://github.com/desireco/zhex/issues/58); host signature:

```text
file.write(contents: String, path: String, append: Bool = false) -> Result<Unit, FileError>
```

Demand: save the proposed line-filter report or a Life generation snapshot; current programs can print but cannot write a file. This story depends on the report workload, not new generics. Create-or-truncate/overwrite when `append` is false or omitted, append with create-if-missing when true; no implicit parent creation, UTF-8 contents with explicit handling of invalid host-supplied argument strings, and resource closure. Errors can leave a partial/truncated file; do not promise atomic replacement. Test existing/missing paths, empty contents, write/close failures and short writes through suitable host seams, and failure propagation/reporting. Preserve error-family rules when composing with console arguments; demonstrate explicit wrapping if families need unification. A separate atomic-save API requires a later portability contract.

Shipped notes: named `file.write`, keeping data-first `contents, path` order — a deliberate asymmetry with path-first reads. Built-ins gained trailing constant defaults as a general checker/eval rule (per-word arity ranges plus constant values, no per-word special case); call sites may omit them, and partial application keeps omitted defaults deferred, matching the user-word rule. The open, write, and close happen inside the operation, so no writable `File` escapes; created files use mode 0644 before umask, and the write path follows symlinks. The stable code is `file-write`, naming the path, the failing phase (`open`, `write`, or `close`), and the operating system's reason; a failed write may leave a partial or truncated file, and a missing parent directory is never created. Invalid-UTF-8 host arguments keep the #57 stance of no new validation. The `file-write` example demonstrating the checked failure path is registered.

**S3. Reduce the edit-distance recurrence.** Start with concrete source words `min(left: Int, right: Int) -> Int` in the algorithm or a sibling module. Demand: replace the manual three-way minimum in `examples/edit-distance.zhex` while preserving exact results. This can ship independently of S1/S2. S1's empty-separator split also permits precomputing code-point lists once instead of repeatedly slicing whole strings in the nested loop; benchmark that change before claiming a speedup. Only promote `int.min`/`int.max` into the core after a second workload demonstrates reuse; a two-line helper alone does not justify more built-ins. If promoted, include MinInt64/MaxInt64 and equal-value cases. Do not add `abs` without specifying MinInt64 overflow, or Float/math APIs without new type design. Life already has sufficient rectangular arrays; no new grid type is needed.

### Phase 2 — reusable collection composition

**S4. Pure-result list transformations.** Shipped as [#60](https://github.com/desireco/zhex/issues/60); checker-polymorphic host signatures (the `T`, `U`, `A` notation describes built-ins, not supported user generic syntax):

```text
list.map(values: List<T>, transform: (T) -> U) -> List<U>
list.filter(values: List<T>, predicate: (T) -> Bool) -> List<T>
list.fold(values: List<T>, initial: A, step: (A, T) -> A) -> A
```

Demand: map and filter line-filter records and fold report totals without bespoke loops. Depends on S1 for the illustrative workload and on extending callable type constraints, not on generic user words. Accept existing explicit partials (`text.trim(_)`), not new lambda syntax. Reject callbacks whose return type is Result in this story. “Pure-result” does not mean effect-free: ordinary callbacks may print or mutate other values. Evaluate arguments once; invoke callbacks once per element left-to-right. Hold the source container's traversal guard, release it on all exits, and preserve side effects already performed before a runtime failure. New map/filter outer lists are fresh, nested values shared; folding may return the explicit accumulator object. Empty input gives empty output or the unchanged initial accumulator. Test nested aliasing, source mutation through aliases, callbacks with effects, inference failures/ambiguous empty collections, limits, and function values imported from modules.

Usage:

```text
flow text.split(" Ada , Grace ", ",")
    list.map(text.trim(_))
    text.join(" / ")
    print
```

Shipped notes: callbacks are existing function values — explicit partials (`text.trim(_)`), word-name partials, or let-bound callables — with no lambda syntax, and they may print or mutate unrelated values. Callbacks run once per element left to right; the source list holds its traversal guard (`collection-iterating`) across every exit, and side effects before a runtime failure remain. `list.map`/`list.filter` return a fresh outer list with shared elements and an unchanged source; `list.fold` returns the explicit accumulator. A callback whose return type is `Result`, or a `Result` fold accumulator, is rejected at check time with `a Result cannot be returned from a callback` under the `unhandled-result` code; no new diagnostic codes were added. The `score-report` example demonstrating the words is registered.

**S5. Short-circuit search and predicates.** Shipped as [#61](https://github.com/desireco/zhex/issues/61), reusing S4's callback contract; host signatures:

```text
list.find(values: List<T>, predicate: (T) -> Bool) -> Option<T>
list.any(values: List<T>, predicate: (T) -> Bool) -> Bool
list.all(values: List<T>, predicate: (T) -> Bool) -> Bool
```

Demand: find the first malformed report record and check whether any/all rows match a rule. Return `None` for no match; empty any is false and empty all is true. Stop immediately when determined, including observable callback effects. Same alias/guard/type rules as S4. Keep this a separate story so ordinary transformation work does not expand into every Enumerable method.

Shipped notes: `list.find` returns built-in `Option.Some` for the first match and `Option.None` otherwise, including for an empty list; `list.any` and `list.all` stop calling the predicate at the first decisive element, observable through callback effects, with empty `any` false and empty `all` true. The traversal guard is held until the answer is determined. No new diagnostic codes were added; the `score-report` example demonstrates these words alongside S4's.

**S6. Deterministic copied sorting.** Shipped as [#62](https://github.com/desireco/zhex/issues/62), reusing S4's callback contract; host signatures:

```text
list.sort(values: List<Int>) -> List<Int>
list.sort(values: List<String>) -> List<String>
list.sort-by(values: List<T>, key: (T) -> Int) -> List<T>
list.sort-by(values: List<T>, key: (T) -> String) -> List<T>
```

These are constrained overloads, not new String relational operators. Demand: rank report records and print a deterministic ordered summary. Stable ascending order; String order is lexicographic Unicode code-point order, without locale collation or normalization. Evaluate keys exactly once in original order before sorting; reject Result keys initially. Fresh outer list, shared elements, unchanged source order. Test ties with record identities, empty input, negative numbers, Unicode, callback counts/effects and alias mutation rejection. No arbitrary comparator, reverse flag, or map reordering until demanded.

Shipped notes: both words return stable ascending copies — a fresh outer list with shared elements and an unchanged source, with equal keys keeping source order; `String` order is Unicode code-point order. `list.sort` accepts `List<Int>` or `List<String>` values; `list.sort-by` orders any list by a key function returning `Int` or `String`, evaluated exactly once per element in source order before any comparison while the source holds its traversal guard. Keys follow the S4 callback contract; a `Result`-returning key is rejected under the existing `unhandled-result` code. No new diagnostic codes were added — the constrained types reuse `argument-type` (`list.sort requires a List<Int> or List<String>, found ...`, `list.sort-by requires a key function, found ...`, `list.sort-by key must return Int or String, found ...`); the `sorting` example demonstrates both words.

**S7. Fallible traversal, separately.** Declined and removed: the owner rejected try-map on 2026-11-14 and asked that [#63](https://github.com/desireco/zhex/issues/63) be removed rather than parked; a select-shaped word over lists and arrays is drafted separately as [#70](https://github.com/desireco/zhex/issues/70). The sketch below is the historical research record, not a live proposal. It had been scoped after S4 and an explicit callback-effect design:

```text
list.try-map(values: List<T>, transform: (T) -> Result<U, E>) -> Result<List<U>, E>
```

Demand: parse all numeric report fields without silently dropping invalid input. Stop at the first failure, preserve its error family/origin, never invoke later callbacks, and do not expose a partial result list. Effects from earlier callbacks remain; no transaction promise. Infer exactly one callback family, reject ambiguity and mixed-family misuse, and integrate top-level propagation, reporters, trace behavior, partials, and source guards. No `List<Result<...>>`, `filter-map` that discards errors, or Result-valued user parameters. A fallible fold can follow only if a concrete workload requires it.

### Phase 3 — optional capabilities after evidence

**S8. Paths and configuration for batch reports.** Stage behind a multi-file report example. Candidate host operations `path.join(base: String, child: String) -> String`, `path.base(path: String) -> String`, `path.extension(path: String) -> String`, and `env.get(name: String) -> Option<String>`. Define OS-native lexical semantics (not URLs), absolute-child behavior, empty/root/trailing-separator cases, invalid environment names/encoding, and missing versus empty values. No existence check hidden in cleanup, no containment guarantee, no process launching. Reserve namespaces only when shipping and assess compatibility with existing sibling modules.

Directory listing/metadata is now researched in the #69 candidates below; the `path.*` words moved there too (2026-11-14 owner decision on #64 — path operations align with the folder-operations vocabulary, and #64 proceeds with `env.*` only). CSV needs a real tabular interchange example and quoting/newline contract; split is not a CSV parser. Temporal extraction/comparison should follow a scheduling/report workload rather than another broad date API pass. A user-facing testing library should start from a failing real Zhex test suite and specify runner discovery, assertions, failure locations and exit status; keep the existing Go executable specification meanwhile.

### Folder operations research (#69, 2026-11-14)

The following three stories come from the [#69 folder-operations research](https://github.com/desireco/zhex/issues/69); the owner triaged them on 2026-11-14 into the approved order S9 → S11 → S10, and they are filed as #73, #74, and #75. All reuse the existing `FileError` family and the not-found/permission-denied/other reason taxonomy established by `file-open`/`file-write`, and stay shallow, deterministic, and explicit about the dangerous cases.

**S9. Folder listing, single-path info, and free space.** Shipped as [#73](https://github.com/desireco/zhex/issues/73); host signatures:

```text
file.list(folder: String) -> Result<List<String>, FileError>
file.exists(path: String) -> Bool
file.size(path: String) -> Result<Int, FileError>
file.free-space(path: String) -> Result<Int, FileError>
```

Demand: the owner's folder vocabulary starts with listing names and sizes and answering "does this path exist" — the most common agent need. The two-word shape (names first, sizes per entry) reuses `List<String>` and `Int` instead of a host-owned `(name, kind, size)` record, which would need new checker machinery for a built-in returning a host record. `file.list` is shallow and returns bare entry names (not paths) in deterministic name order; Go's [os.ReadDir](https://pkg.go.dev/os#ReadDir) sorts entries by filename and its `DirEntry` supplies the cheap type with `Info()` on demand, which is exactly this names-first/sizes-per-entry split (Python's `os.scandir` makes the same split). `file.exists` answers existence; the recommendation is a plain `Bool` where not-found gives false and any other stat failure (permission on a parent, invalid path) raises a runtime diagnostic, matching the plain-collection pattern (ordinary value, runtime diagnostic, opt-in checked form). `file.size` returns the byte size of a regular file; a directory reports 0 with no recursive size. Tests: empty folder, missing folder, file passed as the folder argument, permission failure, deterministic order with mixed-case and digit names, UTF-8 names, empty and nonempty file sizes, `exists` on file/folder/missing/permission paths, flow use, and the one-million-element list limit on huge directories. No array/rectangle machinery is implicated: directory entries are a flat list, not a grid.

Decisions settled in review — the stances are recorded in the shipped notes below: `file.exists` as Bool-plus-runtime-diagnostic versus `Result<Bool, FileError>`; whether listing order is lexical byte order (Go's ReadDir default) or another deterministic order; whether hidden dot entries are included (recommendation: included — filtering is `list.filter` work); stable diagnostic codes for the new failure paths (`file-list`, `file-size`).

Non-goals: no recursive walk, no glob patterns, no entry-kind or symlink flag in v1 (names only), no watching, no trash/undo, no folder sizes, no total/capacity free-space variants.

Shipped notes: `file.exists` took the plain-collection pattern — the plain call is a `Bool` where not-found is `false` and any other stat failure is a runtime diagnostic, and `try`/`catch` selects the checked `Result<Bool, FileError>`; return inference gained the checked-dependent solver case for it. Listing order is Go `os.ReadDir` default lexical byte order, and hidden dot entries are included — filtering is `list.filter` work. The one-million-element limit is enforced before materializing and fails inside the `Result` under the shared `collection-limit` code, the `text.split`/`array.new` precedent. New stable codes `file-list`, `file-exists`, `file-size`, and `file-free-space` reuse the not-found/permission-denied/other reason taxonomy and carry path and reason details like `file-open`/`file-write`. Free space shipped in this story per the owner's 2026-11-14 decision (below), not deferred. No example was registered: every candidate needs a real path with deterministic build-time output, so the example was deliberately deferred until S10's `file.make-folder` could create one deterministically; the `file-work` example below pays that debt.

Free space: researched and deferred, then unparked — the owner decision of 2026-11-14 overruled the deferral ("I don't want it parked. I want it implemented... it should work everywhere") and it shipped in S9 above with the `golang.org/x/sys/windows` platform approach rather than an unsupported-platform diagnostic. The portability research behind the original deferral stands: Go's standard library has no portable free-space call — `syscall.Statfs` is Unix-only and Windows needs the out-of-tree `golang.org/x/sys/windows.GetDiskFreeSpaceEx`; of the six comparison languages only Python's `shutil.disk_usage` is portable stdlib (Ruby needs the Sys::Filesystem gem; Gleam's simplifile, Elm, and Haskell's core/directory have none).

**S10. Destructive and structural folder operations.** Proposed in #69; shipped as [#75](https://github.com/desireco/zhex/issues/75), last in the approved order. Host signatures:

```text
file.make-folder(path: String, parents: Bool = false) -> Result<Unit, FileError>
file.delete(path: String) -> Result<Unit, FileError>
file.delete-folder(path: String, recursive: Bool = false) -> Result<Unit, FileError>
file.move(source: String, target: String, overwrite: Bool = false) -> Result<Unit, FileError>
```

Demand: agents must create work folders, clean up, and rename outputs; today Zhex can read and write files but cannot create, remove, or move anything. Mode parameters follow the owner's #58 append decision (flags over extra words). `file.make-folder` creates one level; `parents: true` creates missing ancestors (Go [os.Mkdir](https://pkg.go.dev/os#Mkdir)/`os.MkdirAll`), mirroring #58's no-implicit-parent-creation stance with an explicit opt-in. `file.delete` removes a file or an empty folder and fails on a nonempty folder (Go `os.Remove` semantics) — the safety default. `file.delete-folder` with `recursive: false` (the default) removes an empty folder and fails otherwise; `recursive: true` removes a whole tree (Go [os.RemoveAll](https://pkg.go.dev/os#RemoveAll): best-effort depth-first, first failure reported, partial deletion possible and documented). The recursive default-off is the safety-critical contract: deleting a tree is always a deliberate, visible `recursive: true` at the call site. `file.move` is rename(2)-based (Go [os.Rename](https://pkg.go.dev/os#Rename)) and therefore moves folders as well as files — stated explicitly, since restricting it to files would fight the syscall; `overwrite: false` (default) fails when the target exists, `overwrite: true` replaces an existing file (atomically where the platform allows) and fails on an existing directory. Cross-device moves fail with the platform's error — no implicit copy fallback in v1 (Python's `os.rename` behaves the same; `shutil.move`'s copy fallback is a separate, explicitly ordered contract). Tests: make single/deep/existing/existing-file collision, delete file/empty folder/nonempty folder/missing, recursive tree with nested content and first-failure reporting, move file/folder/missing source/existing target under both overwrite values, and propagation through `FileError` with reporter/trace coverage. Failures leave earlier effects in place — no transaction promise.

Shipped notes: every review decision settled as recommended — `parents: true` tolerates an already-existing folder (Go `os.MkdirAll`) while `parents: false` fails on an existing target; both delete words shipped (`delete` mirrors `os.Remove`, and `delete-folder(recursive: true)` keeps the dangerous call site greppable); and the move target check's check-then-act window is documented, not solved. The move folder-target contract is pinned explicitly rather than left to divergent syscall outcomes: `overwrite: true` replaces an existing file only, and an existing folder target fails under both overwrite values, so every platform reports one behavior; cross-device moves fail with the platform's error. Mode parameters are built-in trailing constant defaults through the #58 rule — one `builtinTrailingDefaults` entry per word, no per-word special cases. Stable codes are one per word, `file-make-folder`, `file-delete`, `file-delete-folder`, and `file-move` — one more than the research sketch so a `delete-folder(recursive: true)` failure names its own call site — reusing the not-found/permission-denied/other taxonomy. The `file-work` example registered with this story pays S9's deferred-example debt with a deterministic make-folder → write → list → size → free-space → delete round trip.

Non-goals: no copy word (a separate contract), no trash/undo, no atomic-replacement promise beyond what rename itself gives, no permission/mode words, no glob deletion.

**S11. Lexical path words in the file namespace.** Proposed in #69, absorbing the path words the owner redirected out of S8/#64 on 2026-11-14; shipped as [#74](https://github.com/desireco/zhex/issues/74). Host signatures:

```text
file.join(base: String, child: String) -> String
file.base(path: String) -> String
file.extension(path: String) -> String
file.directory-of(path: String) -> String
```

Demand: compose report paths from folder arguments and pick apart user-supplied paths without string surgery. All four are pure lexical operations on the host's native separator — no I/O, no existence checks, no containment guarantee ([filepath.Clean](https://pkg.go.dev/path/filepath#Clean) is lexical cleanup, not symlink resolution), following the pathlib PurePath-versus-Path split the plan already cites. Semantics default to Go's `path/filepath`: `join` concatenates with the OS separator and cleans the result; `base` returns the final element (`"/"` → `"/"`, `""` → `"."`, trailing separators removed); `extension` returns the final-dot suffix of the final element including the dot, empty when there is none; `directory-of` drops the final element (`"."` when there is none). One deliberate divergence to settle: Python's `PurePath` makes an absolute child replace the base (`join("a/b", "/c")` → `/c`) while Go's `filepath.Join` keeps it (`a/b/c`); the recommendation is the Python replace rule, the safer behavior when building paths from untrusted components. Namespace: `file.*` per ownership rule 1 (external filesystem work under `file`) — a second `folder.*` namespace for a handful of words fragments the vocabulary, `folder` stays out of the reserved-module list, and no `path.*` namespace is reserved, which resolves #64's redirected words without new reservation work. Tests: absolute-child, empty, root, and trailing-separator cases, dotfile extension behavior, lexical `..` cleaning, `directory-of`/`base` round-trips, and that no word touches the filesystem.

Decisions to settle: absolute child replaces versus concatenates; whether `extension` treats a leading-dot name like `.bashrc` as extension `.bashrc` (Go) or no extension (common convention); whether a fifth word (lexical `file.clean`) is wanted now (recommendation: no — `join` already cleans).

Shipped notes: the absolute child replaces the base — the recommended Python rule — so `file.join("a/b", "/c")` is `/c`; everything else is Go `path/filepath` on the host's native separator, with `join` cleaning lexically. `extension` keeps Go's `filepath.Ext` answer for leading-dot names (`.bashrc` → `.bashrc`), flagged in the claim for owner review; flipping to the Python/Rust no-extension convention stays a one-line change. No fifth word: `join` already cleans. All four return plain `String` — no `Result`, no filesystem access, no new diagnostic codes. One Go quirk is pinned in the tests and stated in the reference: a path ending in a separator has no final element, so `file.directory-of("/a/b/")` is `/a/b` while `file.base("/a/b/")` is `b`. The `paths` example — composing report paths and decomposing them without touching the disk — is registered, raising the example corpus to 28 (29 with #75's `file-work`).

Non-goals: no URL paths, no symlink resolution, no normalization to an absolute form, no `path.*` namespace, no filesystem access of any kind.

**S12. Lists and arrays: select by predicate.** Frozen by the owner interview on 2026-11-14 and shipped as [#70](https://github.com/desireco/zhex/issues/70). Host signatures:

```text
list.select(values: List<T>, predicate: (T) -> Bool) -> List<T>
array.select(values: Array<T>, predicate: (List<T>) -> Bool) -> Array<T>
```

Demand: keep the elements a predicate accepts, unmodified. The owner's frozen decision: "select takes a record, runs through expression and returns boolean, true or false, true is selected and those elements, unmodified are returned back." The previously drafted Option-returning transform was explicitly declined the same day, and column selection stays deferred until a workload demands it. Callbacks follow #60's contract — explicit partials and word names only, captured arguments evaluated once, predicate invoked once per element/row left to right, the traversal guard held and released on all exits, effects before a runtime failure observable — except that `select` visits every element: no early exit. Both words build a fresh outer collection with shared elements and leave the source unchanged; empty input and all-false give an empty list / zero-row array, all-true gives a fresh copy, order is preserved.

Shipped notes: the rank finding from `arrayShape` in `internal/lang/collections.go` — the interpreter supports no rank-1 arrays (rank 2..32 is enforced), so the ticket's "rank 1 follows the list shape" branch is moot. `array.select` is rank-2 only: the predicate receives one row per call in row-major order as a fresh `List<T>` (a copy, so an append inside the predicate cannot overwrite the source's backing store), kept rows are returned unmodified with the rectangle (row width) preserved, and rank 3+ reports `array.select requires a rank-2 array, found rank N` at runtime under `array-rank` — same rationale as deferred columns, revisit on demand. `list.select` coexists with `list.filter` by the owner's conscious redundancy decision. Result-returning predicates are rejected at check time before any effects, like every #60 callback. No new diagnostic codes. No standalone example: the existing `collections` example gained a `select` demonstration instead, so the corpus count is unchanged.

Non-goals: no Option- or Result-returning transforms, no column selection, no in-place mutation, no short-circuit or fold variants, no lambda syntax, no deep copy.
### Document libraries research (#66/#67, 2026-11-14)

The [#66 document research](https://github.com/desireco/zhex/issues/66) (JSON, YAML, and XML as native data) and the [#67 generation research](https://github.com/desireco/zhex/issues/67) (Markdown and HTML around Buffer) completed on 2026-11-14. Six owner decisions from the research window are recorded verbatim-short and baked into the candidates below:

1. JSON heterogeneous trees: design the tree type now — filed as [#76](https://github.com/desireco/zhex/issues/76).
2. YAML: stay dependency-free — deferred indefinitely.
3. TOML: the preferred future candidate if the dependency policy ever changes.
4. XML typed decode: the common `Error` family with `xml-*` codes.
5. Markdown paragraphs: verbatim — no normalization.
6. HTML: `html.raw` included from v1, unmistakably unsafe — the module guarantee becomes "escaped unless raw".

Story numbering continues the #69 sequence. S13 was the JSON heterogeneous-tree slot; the owner's mid-research decision filed it as #76, its design lives there, and it is not repeated here. It has since shipped: `json.parse` parses into the host-predeclared `Json` tree, `json.encode` accepts trees with byte-faithful round trips, and the `json-tree` example is registered. S12 shipped, S14 shipped as [#77](https://github.com/desireco/zhex/issues/77) with tables added by owner decision, and S15 remains a candidate awaiting owner triage.

Fit, defer, and exclude matrix:

| Area | Verdict | Record |
| --- | --- | --- |
| JSON typed decode | Fit — already shipped | [#33](https://github.com/desireco/zhex/issues/33) shipped `json.decode`/`json.encode` with strict fields, defaults, and explicit Results; no new work needed. |
| JSON heterogeneous trees | Shipped | Owner decision: the tree type was designed now and filed as #76; it shipped there as `json.parse` with the host-predeclared `Json` tree and the `json-tree` example. |
| YAML | Defer indefinitely | Owner decision: stay dependency-free. [goccy/go-yaml](https://github.com/goccy/go-yaml) was the researched candidate (MIT, active); [yaml.v3](https://github.com/go-yaml/yaml), archived 2025, is the cautionary precedent. |
| TOML | Defer — preferred future candidate | Owner note: the preferred candidate if the dependency policy ever changes. Go's stdlib has no TOML either; [BurntSushi/toml](https://github.com/BurntSushi/toml) and [pelletier/go-toml/v2](https://github.com/pelletier/go-toml) are the maintained modules — equally dependency-carrying, better format determinism, healthier ecosystem. |
| XML typed decode | Shipped | [#79](https://github.com/desireco/zhex/issues/79) shipped `xml.decode` dependency-free on stdlib [encoding/xml](https://pkg.go.dev/encoding/xml); see the S12 section below. |
| XML DOM, mixed content, namespaces-as-data | Defer behind the tree-type wall | #76 now partially addresses the wall by proving the host-predeclared tree pattern for JSON; reopen only when a concrete mixed-content or namespaces-as-data XML workload exists after #76 lands. |
| Markdown generation | Fit — S14 candidate | Pure-Zhex sibling module; see S14 below. |
| HTML generation | Fit — S15 candidate | Pure Zhex plus one promotion candidate; see S15 below. |
| Markdown parsing | Exclude | Generation is the workload; [goldmark](https://github.com/yuin/goldmark) would add a dependency and a parsing contract this research does not support. |
| Template engines | Exclude | No template syntax; composition stays Zhex words over fragments. |

**S12. XML typed decode.** Shipped as [#79](https://github.com/desireco/zhex/issues/79) (this S12 label collides with the select-by-predicate story numbered S12 in the shipped table above, which is #70; renumbering is an owner decision left open here). Host signature:

```text
xml.decode(text: String, defaults: Record) -> Result<Record, Error>
```

Owner decision baked in: the common `Error` family with `xml-*` codes, not a new error family. Shipped 2026-11-14 with codes `xml-syntax`, `xml-type`, `xml-field`, `xml-range`, and `xml-depth`; the settled decisions from the issue body: attributes map onto record fields verbatim with collision-fails-decode, repeated elements decode into `List<T>`, absent elements use defaults or `Option<T>`, strict unknown-element rejection, predefined entities only with DTD/entity definitions rejected, no network ever, depth 256 and element limits on the JSON precedents, prefixed element names rejected, text-only elements decoding as `String`/`Int`. The `xml-config` example is registered.

Demand: the #66 typed-configuration workload — read XML documents into declared records with the same strictness and immediate Result handling as `json.decode`, dependency-free on Go's [encoding/xml](https://pkg.go.dev/encoding/xml).

Acceptance criteria:

- Strict fields mirroring `json.decode`: unknown elements or attributes are rejected, and supplied defaults fill absent optional elements.
- Attributes and element text are the only mapped content in v1; repeated sibling elements decode into `List<T>`, and an absent optional element decodes to its default or `Option<T>`.
- Namespaces: prefixed names are rejected in v1 rather than silently flattened.
- No DTD or entity definitions are processed, and no network access ever happens during parsing.
- Depth and size limits and the failure contract follow the json codec precedent, with input position in the diagnostic where available; codes are stable `xml-*` names under the common `Error` family.
- Tests first per the standing story rules; the new `xml` namespace goes through the [vocabulary-change checklist](vocabulary-checklist.md), including module-name reservation and editor word lists.

Decisions to settle: the concrete `xml-*` code names and message shapes; whether defaults apply to absent elements only or also to empty text; how an element carrying both attributes and text maps onto a record expecting one of them.

Non-goals: no XML encoding or serialization, no DTD/entity expansion, no XPath, no mixed content or namespaces-as-data (deferred behind the tree-type wall above), no streaming.

**S14. Markdown generation module.** Filed from the #67 research as [#77](https://github.com/desireco/zhex/issues/77) and shipped as `examples/modules/md-report/md.zhex` with the registered `modules/md-report/main` example. A pure-Zhex sibling module — no host words, no checker or host changes, so the editor keyword lists do not apply (sibling-module words are not builtins and the #71 drift test does not cover them). Module words:

```text
md.heading(level: Int, text: String) -> String
md.paragraph(text: String) -> String
md.bullet-list(items: List<String>) -> String
md.ordered-list(items: List<String>) -> String
md.link(text: String, target: String) -> String
md.code-block(language: String, lines: List<String>) -> String
md.table(headers: List<String>, rows: List<List<String>>) -> String
```

Owner decisions baked in: fragments, not appenders — every word returns a complete `String` fragment and never mutates a buffer, so `Buffer` stays the caller's accumulator; and the verbatim paragraph policy — caller text is emitted as written, with no normalization.

Demand: the #67 report workload — compose native records and lists into a Markdown document through Buffer, without a renderer or template engine; fragment-returning words compose data-first in flows.

Acceptance criteria:

- CommonMark core subset only; the excluded extensions (tables, footnotes, task lists) are listed in the module's documentation.
- Per-position escaping: heading and paragraph text escape Markdown metacharacters, link text and link URLs each get position-correct escaping, and code blocks select a fence longer than any backtick run in the content.
- Verbatim paragraphs per the owner decision: no rewrapping, trimming, or blank-line insertion beyond what separates the fragment's own blocks.
- Empty and multiline inputs are defined per word; nested composition (a link inside a list item) is tested.
- Fixtures cover the #67 list: Unicode, ampersands, angle brackets, quotes, Markdown metacharacters, multiline code, empty content, and shared Buffer aliases at the call site.
- Ships as a concrete sibling module (`use md`) with concrete signatures — no checker or host changes.

Decisions settled in review and pinned in the module: it lives beside the registered two-file example at `examples/modules/md-report/` (`main.zhex` plus `md.zhex`, resolved by the ordinary sibling rule); heading levels clamp into 1–6; `md.link` always percent-encodes spaces and parentheses and neutralizes `javascript:`/`data:` targets by percent-encoding every colon; `md.code-block` scans its lines for the longest backtick run and emits a fence of `max(3, run + 1)` backticks with the language tag verbatim; `md.table` (owner decision 2026-11-14: tables are in) requires every row to match the header width — a mismatching row fails the interpreter's `list-range` diagnostic before any table renders; empty input composes to the empty fragment and blank-line separation stays caller-side via `text.join(fragments, "\n\n")`.

Non-goals: no Markdown parsing, no HTML output, no further extension syntax beyond tables, no text normalization, no Buffer mutation, no template engine.

**S15. HTML generation module.** Candidate from the #67 research, awaiting owner triage; not filed as an issue. A pure-Zhex sibling module plus one promotion candidate: `html.escape-text` as a host word backed by Go's [html](https://pkg.go.dev/html) escaping, because correct context-free text escaping is subtle enough to earn the host boundary. Proposed module words:

```text
html.escape-text(text: String) -> String
html.element(tag: String, attributes: Map<String, String>, content: String) -> String
html.page(title: String, body: String) -> String
html.raw(markup: String) -> String
```

Owner decision baked in: `html.raw` is included from v1, named unmistakably unsafe, and documented as bypassing every escaping property — the module guarantee becomes "escaped unless raw".

Demand: the #67 report workload rendered as HTML — element and page composition from native records and lists, escaped text by default, and a deliberate, greppable raw escape hatch.

Acceptance criteria:

- Every text and attribute value is escaped in every position unless the caller passes it through `html.raw`; `html.raw` returns its argument unchanged, and its reference documentation states it bypasses every escaping property.
- URL scheme policy: `javascript:` and `data:` URLs are rejected in URL-valued positions instead of emitted.
- `html.element` emits one well-formed element with escaped attribute values; `html.page` emits a complete minimal HTML document.
- Fixtures cover the #67 list: Unicode, ampersands, angle brackets, quotes, empty content, and nested composition.
- If the owner approves the `html.escape-text` promotion with this story, it follows the [vocabulary-change checklist](vocabulary-checklist.md); otherwise it stays a module word.

Decisions to settle: the attribute representation (`Map<String, String>` versus pre-escaped `List<String>` pairs); which attribute names are URL-checked (`href`/`src` only or a wider set) and the exact blocklist beyond `javascript:`/`data:`; whether `html.escape-text` ships here or as its own small story.

Non-goals: no HTML parsing, no DOM, no template language, no CSS/JavaScript generation, no sanitization claim (`html.raw` is a documented bypass, not a sanitizer), no network.

## Deferred and excluded scope

- **HTTP client/server:** optional capability work, not Phase 1. [Go Client.Do](https://pkg.go.dev/net/http#Client.Do) illustrates why status codes are response data while transport failures are errors. Define response records, closure, timeout, redirects, body caps, headers and text/binary representation before introducing a getter. Server lifecycle is another story.
- **Process execution:** optional host library. [Go exec](https://pkg.go.dev/os/exec) and [Python subprocess](https://docs.python.org/3/library/subprocess.html) support explicit argument vectors. Specify launch failure versus nonzero-exit data, timeout, capture limits and encoding; no implicit shell-string API.
- **Regex:** defer until literal operations fail a real parsing task. [Go regexp](https://pkg.go.dev/regexp) provides input-linear matching, but compiled-pattern values, limits and code-point match positions need a Zhex contract.
- **Randomness, hashing, binary encodings:** define a byte representation and capability boundaries first where needed. [Python secrets](https://docs.python.org/3/library/secrets.html) separates security tokens from simulation randomness. Do not offer an ambiguous universal random function.
- **Generic source libraries, installed modules, package management:** separate language/distribution work. Monomorphic source helpers and generic built-in checking are sufficient for these early phases.
- **No implicit coercion, automatic currying, lazy collection semantics, arbitrary precision, generic equality, Result-erasing convenience, or giant compatibility layer.** None is needed to validate the first report workflow.

## Decision and completion boundary

Recommended first batch: S1 and S2, with S3 independently improving the existing algorithm example. Then S4, followed by evidence-driven S5/S6; the deliberately separate S7 was later declined and removed with #63. S8 and other optional capabilities stay proposals.

The owner approved story creation and one-by-one review. The stories were first filed in Inbox with no status, then explicitly moved to Todo at the owner's request; unresolved edge contracts stay explicit for review. Research and story creation do not implement these APIs or change the current reference; S1 (#57) and S2 (#58) have since shipped and are recorded in the inventory above, S3 (#59) shipped as source-level helper words without new host names, and S4 (#60) with S5 (#61) have since added the list callback words, and S6 (#62) the two list sorting words, to that inventory; S9 (#73) from the folder-operations research added the four file inspect words, and S10 (#75) the four destructive file words.

| Order | Story | Dependency |
| --- | --- | --- |
| S1 | [#57 — Literal text split/join and prefix/suffix](https://github.com/desireco/zhex/issues/57) — shipped | Full line-filter application still open |
| S2 | [#58 — Persist a text report](https://github.com/desireco/zhex/issues/58) — shipped | Atomic-save needs a later portability contract |
| S3 | [#59 — Simplify edit-distance minimum](https://github.com/desireco/zhex/issues/59) — shipped | Local helper words; `int.min`/`int.max` promotion awaits a second workload |
| S4 | [#60 — List map/filter/fold](https://github.com/desireco/zhex/issues/60) — shipped | #57 workload and existing callable support |
| S5 | [#61 — Short-circuit find/any/all](https://github.com/desireco/zhex/issues/61) — shipped | #60 callback contract |
| S6 | [#62 — Stable copied sort/sort-by](https://github.com/desireco/zhex/issues/62) — shipped | #60 callback contract |
| S7 | [#63 — Explicit fallible try-map](https://github.com/desireco/zhex/issues/63) — declined and removed | #60 plus callback-effect design; declined 2026-11-14, #70 drafts the replacement |
| S8 | [#64 — Paths/environment for batch reports](https://github.com/desireco/zhex/issues/64) | #57/#58 and a multi-file workload; later phase |
| S9 | [#73 — File: inspect — list, exists, size, and free space](https://github.com/desireco/zhex/issues/73) — shipped | #69 research; executable example deferred until S10's `file.make-folder` |
| S11 | [#74 — File: lexical path words — join, base, extension, directory-of](https://github.com/desireco/zhex/issues/74) — shipped | #69 research; the `paths` example registered |
| S10 | [#75 — File: destructive operations — make-folder, delete, delete-folder, move](https://github.com/desireco/zhex/issues/75) — shipped | #69 research; registered the deferred S9 example as `file-work` |
| S13 | [#76 — Json: host-owned tree type with json.parse](https://github.com/desireco/zhex/issues/76) — shipped | #66 document research; owner-frozen design; `json-tree` example registered |
| S12 | [#70 — Lists and arrays: select by predicate](https://github.com/desireco/zhex/issues/70) — shipped | #60 callback contract; `list.select` coexists with `list.filter` per owner decision |
| S14 | [#77 — Markdown: report module as a pure-Zhex sibling module](https://github.com/desireco/zhex/issues/77) — shipped | #66/#67 research; owner-locked fragments and verbatim paragraphs, tables added by owner decision; no host words, so no inventory or editor changes; `modules/md-report/main` example registered |
| — | [#72 — Language: while loop and read-line EOF reason](https://github.com/desireco/zhex/issues/72) — shipped | #65 prerequisite, ordered last in the queue; `while` is syntax (keyword), not a host word, so the inventory count is unchanged |
| — | [#79 — XML: typed decode](https://github.com/desireco/zhex/issues/79) — shipped | #66 document research; common Error family with `xml-*` codes; `xml-config` example registered |

No story is In Progress merely because it appears in this order. #30 tracks completion of research and backlog preparation; each story follows the normal claim, implementation, verification, and close-last workflow when selected.

## Additional dogfooding exploration

After approving the eight library stories, the owner also approved [#65 — a Zhex-written REPL front end](https://github.com/desireco/zhex/issues/65) as a bounded investigation/prototype. Zhex would own prompt, buffer, and dispatch policy over the existing Go Session engine; lexing, checking, evaluation, and terminal machinery stay host-owned. The story must examine a minimal typed session bridge, indefinite input without recursive stack growth, EOF/cancellation, and shared program/source input. Transcript parity comes before any production cutover, with terminal parity a separate later gate. This is not a promise to rewrite the interpreter in Zhex. It was filed in Inbox and then moved to Todo at the owner's request; it can inform library gaps without depending on every earlier story. Its two language prerequisites shipped as S13 ([#72](https://github.com/desireco/zhex/issues/72)): the statement-only `while` loop (Unit, no `break`/`continue`, no iteration limit by design) and `read-line`'s additive EOF `reason` detail (`end-of-file` versus `read-failed`), keeping the `Result<String, InputError>` signature unchanged.
