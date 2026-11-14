# Session code review — issue #56

Status: independent review completed; eight actionable findings were repaired and rechecked. The frozen review baseline remains unchanged. Release verification is recorded below and on [issue #56](https://github.com/desireco/zhex/issues/56).

## Frozen scope

- Start exclusive: `0db104ac8983987fe26901d234e264f57f36b34e`.
- End inclusive: `0b21a0c041a8689269a140941631fa1520521d5e` (full #55 shipped release).
- Exact diff: `git diff 0db104ac8983987fe26901d234e264f57f36b34e...0b21a0c041a8689269a140941631fa1520521d5e`; baseline is an ancestor, so this equals the two-dot content comparison.
- 202 commits, 180 changed paths. Includes concurrent `8bbbe4e`, its `93b4b7a` revert, and Astro `a13ab08`; follow-up repairs are recorded separately and do not move the baseline.
- Applicable standards: AGENTS.md, accepted language contract, error-handling guidance and relevant approved design notes. Review skills: code-review (Standards and Spec axes), code-review-and-quality (correctness, clarity, architecture, security, performance).

## Ownership and method

Sol: independent lexer/parser/checker/type/effect/closure/module/REPL semantics and consolidated ledger. Terra: runtime/JSON/temporal/resource self-audit, focused fixes, sole main index/build/release. Luna: docs/site/examples and build/editor evidence. Complementary review results are recorded with their author; author testing is not a substitute for independent signoff. Parent coordinates any additional cross-review.

Read implementation and behavior tests; probe accepted and rejected programs with actual checked execution, source diagnostics and bounded failure behavior. Record reproducible findings, meaningful regression checks and fix hashes. Keep Standards and Spec conclusions separate; avoid speculative style churn. Final acceptance requires no known unresolved actionable finding, not proof that bugs are impossible.

## Findings

| ID | Area / reproduction | Repair | Independent verification |
| --- | --- | --- | --- |
| F1 | A host trace writer that panics escaped `runtime.emit`, violating the observational tracing contract and terminating execution. | `f8f354a` isolates only the trace write. | Sol reviewed the narrow recovery, passed its regression, and removed recovery in an isolated archive: the regression failed/panicked as expected. Main implementation files were untouched by the mutation. |
| F2 | Results reference presented plain list/array operations as always returning Results. | `fe64d9d` separates directly fallible operations from opt-in checked collection mode and explains that `try` unwraps success. | Luna reproduced the plain partial's `Unit`/`try` rejection and ran the checked-wrapper snippet; Sol checked final table/prose against actual typing. |
| F3 | Syntax examples still claimed collections were unnecessary despite new list/array/map examples. | `fe64d9d` corrects the concrete/monomorphic description. | Luna and Sol reviewed the corpus and corrected text. |
| F4 | The accepted call plan still claimed shipped syntax was not implemented. | `fe64d9d` marks it historical/shipped at `0b21a0c`, links the current guide, and fixes stale example/delivery wording. | Independent cross-review by Luna and Sol. |
| F5 | `--trace` on a flow creating/invoking a user-word partial panicked because the checked first input type was absent. | `f8d6981` records the checked input type; `49c7d7a` adds builtin-partial flow coverage. | Sol reproduced the panic with the shipped baseline, then verified flow creation/invocation trace events and output `12` on the repaired interpreter. |
| F6 | Repeated failed REPL entries retained inaccessible new binding/value roots in the persistent frame; successful block locals/shadowing could also accumulate. | `d7adf17` prunes unreachable frame keys and flattens visible scopes while preserving binding identities. | Sol's isolated baseline repro retained eight roots after eight failures and passed with the repair. Regression tests preserve existing variable/shared-object mutations, old captured values, and bound scope depth. |
| F7 | An Error-valued callable flow step bypassed the direct-call/user-word guard and stored a concrete caught ParseError in `List<Error>` without `error.wrap`. | `6cc1135` applies the conservative Error-flow rule to callable bindings. | Sol reproduced storage/inspection bypasses with the baseline; both now reject before effects. Regressions include builtin inspector functions and a valid explicitly wrapped direct call. |
| F8 | A persistent REPL word/default/partial failure could underline an unrelated line in the current entry because all entries reused the `repl` source-map key. | `e3becd9` retains immutable parsed-source identity on positions and uses it for excerpts. | Sol reproduced division-by-zero underlining `99`; repaired output underlines original `1 / 0`. Source lifetime/copy/report paths reviewed; regressions cover old words, partials, record/word defaults and current-entry errors. |

F6 and F8 are retained pre-existing patterns in code touched by the session, rather than claims that the last partials commit introduced them. They are included because the owner requested repair of every actionable issue found in the reviewed implementation.

No confirmed actionable finding remains open at this checkpoint.

## Standards review

AGENTS.md is the repository's coding/workflow standards source; no separate CONTRIBUTING or CODING_STANDARDS file exists. Focused conventional repairs preserve language grammar/API scope, stable diagnostic first lines/codes, whole-program checking, main/index ownership and required gates. Public truth inconsistencies F2–F4 were corrected. The review does not turn file-size or style heuristics into a broad unrelated refactor.

Inventory covers all 202 commits/180 changed paths, including the transient deletion/revert and final cumulative state. Generated parser/lock artifacts are assessed through their source/generation inputs, corpus/query tests and dependency checks. This is not a claim that every intermediate historical checkout was independently rebuilt.

## Spec review

- Sol independently reviewed lexer/parser/block/call boundaries; named types, aliases/choices/privacy and structural depth; binding/Result/Error obligations; whole-program and REPL inference; closure capture/default/invocation/effect contracts; module canonical identities; source diagnostics and trace boundaries. Earlier feature review evidence was supplemented by new audit repros, not used as sole signoff.
- Runtime cross-check: Terra inspected JSON, temporal, aliases/resources/reporting and performed race/fuzz checks. Sol additionally read the JSON schema/parser/clone/encode paths, temporal constructor/parse/arithmetic/civil-transition code, reporting snapshots and runtime/REPL/trace boundaries; independently reproduced/rechecked F1/F5/F6/F7/F8.
- Luna independently reviewed public/repository docs, canonical examples/output, Astro docs/examples/navigation, site-data generation/link checks and Pages wiring; Sol cross-checked the fixes and the rendered/source/output evidence. Editor source/generated artifacts are covered by the shipped grammar/query/corpus/example and Vim/Neovim/VS Code checks, followed by final release verification.
- No new language feature, public Never type, opaque representation, automatic currying, hidden Result relaxation or allocation/OOM promise was added by the repairs.

## Verification and limitations

Audit evidence:

- Sol `FuzzCompile`: 30,461 executions in a 10-second request; 299 total seeds; no failure. Terra independent 15-second run: 36,424 executions; no failure. Inputs are bounded at 16 KiB by the fuzz harness; these runs are not an exhaustive proof.
- Full Go tests, `go vet ./...`, `go test -race ./...`, and `just test` passed through `e3becd9` per the integration run. Targeted repaired regressions and original repros were independently rechecked as described above.
- The trace recovery mutation was performed only in `/tmp`; source/program probes and the failed-entry retention test used isolated archives. Sol edited only this ledger/design documentation in the shared checkout.
- `npm audit --audit-level=high --json` reported zero advisories for the locked site graph (294 dependency entries).
- `govulncheck` v1.9.0, scanning all four root packages with Go 1.27.3, reported zero reachable symbol/package vulnerabilities. It additionally reported two module/toolchain-only advisories not called by this code: [GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859) (`net/http`, toolchain fix Go 1.27.4) and [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024) (`x/sys/windows.NewNTUnicodeString`, fixed x/sys v0.44.0). These are recorded as scan limitations/information, not an assertion that the modules contain no advisories. No host toolchain was modified. The Windows report was independently checked against the official Go vulnerability database.
- Current behavior intentionally retains documented limits/exclusions (for example checked intrinsic guard scope, supported JSON shapes and temporal year range). Unsupported features are not audit failures merely because they are not implemented.

Release checkpoint at `ea26c53bcbdcb9c44dc99762e9de9d8da247651a`: final `just test` and `go vet` passed, along with the same-code race run and shipped editor checks. The fresh binary reported this exact revision and `vcs.modified=false`; all 23 canonical outputs and repaired flow-trace/Error-flow/REPL-source probes passed independently. All 23 rendered primary/module sources, outputs and downloads matched; all 28 generated pages passed HTTP, single-H1 and horizontal-overflow checks at 320/1440 pixels. The repaired Results reference was present in the built site.

Pages [43569707032](https://github.com/desireco/zhex/actions/runs/43569707032) and editor [43569720342](https://github.com/desireco/zhex/actions/runs/43569720342) passed for that checkpoint. The final ship comment on [#56](https://github.com/desireco/zhex/issues/56) records live verification and the final published revision after this evidence note. No claim is made that all possible defects or performance pathologies have been eliminated.

## Changed-path inventory

Each path below is in the frozen diff. Generated files are assessed through generator/source review plus regeneration/corpus evidence, rather than assumed correct from their generated label.

- `.github/workflows/pages.yml`
- `.gitignore`
- `AGENTS.md`
- `README.md`
- `ROADMAP.md`
- `cmd/zhex/block_rhs_pty_linux_test.go`
- `cmd/zhex/block_rhs_test.go`
- `cmd/zhex/main.go`
- `cmd/zhex/main_test.go`
- `cmd/zhex/multiline_pty_linux_test.go`
- `cmd/zhex/multiline_test.go`
- `cmd/zhex/repl_editor.go`
- `cmd/zhex/repl_editor_test.go`
- `cmd/zhex/repl_pty_linux_test.go`
- `docs/call-syntax-plan.md`
- `docs/diagnostics-plan.md`
- `docs/error-handling-guidance.md`
- `docs/error-handling-research.md`
- `docs/first-milestone.md`
- `docs/getting-started.md`
- `docs/initial-language.md`
- `docs/language-presentation-research.md`
- `docs/rich-values-plan.md`
- `docs/syntax-examples.md`
- `docs/tests.md`
- `docs/website.md`
- `editors/testdata/highlighting.zhex`
- `editors/tree-sitter-zhex/grammar.js`
- `editors/tree-sitter-zhex/queries/highlights.scm`
- `editors/tree-sitter-zhex/src/grammar.json`
- `editors/tree-sitter-zhex/src/node-types.json`
- `editors/tree-sitter-zhex/src/parser.c`
- `editors/tree-sitter-zhex/test/corpus/aliases.txt`
- `editors/tree-sitter-zhex/test/corpus/choices.txt`
- `editors/tree-sitter-zhex/test/corpus/collections.txt`
- `editors/tree-sitter-zhex/test/corpus/error_handling.txt`
- `editors/tree-sitter-zhex/test/corpus/flow_arguments.txt`
- `editors/tree-sitter-zhex/test/corpus/generic_choices.txt`
- `editors/tree-sitter-zhex/test/corpus/maps.txt`
- `editors/tree-sitter-zhex/test/corpus/multiline_named_blocks.txt`
- `editors/tree-sitter-zhex/test/corpus/record_defaults.txt`
- `editors/tree-sitter-zhex/test/corpus/syntax.txt`
- `editors/tree-sitter-zhex/test/highlight/basics.zhex`
- `editors/tree-sitter-zhex/test/highlight/error_handling.zhex`
- `editors/vim/syntax/zhex.vim`
- `editors/vim/test/highlight.vim`
- `editors/vscode/language-configuration.json`
- `editors/vscode/syntaxes/zhex.tmLanguage.json`
- `editors/vscode/test/highlight.test.cjs`
- `editors/zed/extension.toml`
- `editors/zed/languages/zhex/brackets.scm`
- `editors/zed/languages/zhex/highlights.scm`
- `examples/calls.zhex`
- `examples/choices.zhex`
- `examples/collections.zhex`
- `examples/edit-distance.zhex`
- `examples/exit-status.zhex`
- `examples/json-config.zhex`
- `examples/life.zhex`
- `examples/maps.zhex`
- `examples/partials.zhex`
- `examples/records.zhex`
- `examples/recovery.zhex`
- `examples/temporal.zhex`
- `examples/word-defaults.zhex`
- `go.mod`
- `go.sum`
- `internal/lang/aliases_test.go`
- `internal/lang/args_test.go`
- `internal/lang/block_rhs_test.go`
- `internal/lang/buffer_test.go`
- `internal/lang/catalog.md`
- `internal/lang/check.go`
- `internal/lang/check_functions.go`
- `internal/lang/check_types.go`
- `internal/lang/civil_time.go`
- `internal/lang/civil_time_test.go`
- `internal/lang/codes.go`
- `internal/lang/codes_test.go`
- `internal/lang/collections.go`
- `internal/lang/diagnostics_test.go`
- `internal/lang/edit_distance_test.go`
- `internal/lang/effect_constraints.go`
- `internal/lang/effects.go`
- `internal/lang/eval.go`
- `internal/lang/examples_test.go`
- `internal/lang/features_test.go`
- `internal/lang/file_test.go`
- `internal/lang/files.go`
- `internal/lang/flow_args_test.go`
- `internal/lang/function_types.go`
- `internal/lang/function_types_test.go`
- `internal/lang/hints.go`
- `internal/lang/host.go`
- `internal/lang/indexed_writes_test.go`
- `internal/lang/infer_parameters.go`
- `internal/lang/infer_returns.go`
- `internal/lang/inferred_effects_test.go`
- `internal/lang/json_codec.go`
- `internal/lang/json_test.go`
- `internal/lang/lexer.go`
- `internal/lang/life_test.go`
- `internal/lang/maps.go`
- `internal/lang/maps_test.go`
- `internal/lang/modules_test.go`
- `internal/lang/multiline_test.go`
- `internal/lang/named_record_args_test.go`
- `internal/lang/parameter_inference_test.go`
- `internal/lang/parser.go`
- `internal/lang/partials_test.go`
- `internal/lang/plain_collection_test.go`
- `internal/lang/plain_index_test.go`
- `internal/lang/record_defaults_test.go`
- `internal/lang/reporting.go`
- `internal/lang/reporting_test.go`
- `internal/lang/result_words_test.go`
- `internal/lang/return_inference_test.go`
- `internal/lang/rich_values_test.go`
- `internal/lang/session.go`
- `internal/lang/session_test.go`
- `internal/lang/simple_calls_test.go`
- `internal/lang/syntax.go`
- `internal/lang/temporal.go`
- `internal/lang/temporal_test.go`
- `internal/lang/trace.go`
- `internal/lang/trace_test.go`
- `internal/lang/type_constraints.go`
- `internal/lang/type_constraints_test.go`
- `internal/lang/word_defaults_test.go`
- `justfile`
- `site/astro.config.mjs`
- `site/docs.tmpl`
- `site/errors.tmpl`
- `site/examples.tmpl`
- `site/llms.txt`
- `site/manifesto.tmpl`
- `site/package-lock.json`
- `site/package.json`
- `site/roadmap.tmpl`
- `site/scripts/finish.mjs`
- `site/scripts/verify.mjs`
- `site/src/components/DocsNav.astro`
- `site/src/content.config.mjs`
- `site/src/content/docs/buffers.md`
- `site/src/content/docs/calls.md`
- `site/src/content/docs/choices.md`
- `site/src/content/docs/collections.md`
- `site/src/content/docs/console.md`
- `site/src/content/docs/control-flow.md`
- `site/src/content/docs/diagnostics.md`
- `site/src/content/docs/editors.md`
- `site/src/content/docs/files.md`
- `site/src/content/docs/getting-started.md`
- `site/src/content/docs/iteration.md`
- `site/src/content/docs/json.md`
- `site/src/content/docs/maps.md`
- `site/src/content/docs/modules.md`
- `site/src/content/docs/records.md`
- `site/src/content/docs/repl.md`
- `site/src/content/docs/results.md`
- `site/src/content/docs/syntax.md`
- `site/src/content/docs/temporal.md`
- `site/src/content/docs/text.md`
- `site/src/content/docs/values.md`
- `site/src/content/docs/words-and-flows.md`
- `site/src/layouts/Base.astro`
- `site/src/pages/docs.astro`
- `site/src/pages/docs/[slug].astro`
- `site/src/pages/errors.astro`
- `site/src/pages/examples.astro`
- `site/src/pages/index.astro`
- `site/src/pages/manifesto.astro`
- `site/src/pages/roadmap.astro`
- `site/styles.css`
- `tools/build-site/main_test.go`
- `tools/build-site/manifesto_test.go`
- `tools/site-data/main.go`
- `tools/site-data/main_test.go`
- `tools/site-data/manifesto.go`
- `tools/site-data/manifesto_test.go`

## Commit inventory

```text
8bbbe4e docs: record module-growth items as future work
93b4b7a Revert "docs: record module-growth items as future work"
a13ab08 feat: migrate website to Astro and organize documentation
64d4e75 feat: declare Result-returning words
f898199 feat: propagate failures through fallible words
e8c3a21 feat: recover Result failures with catch blocks
85f1751 feat: expose structured error inspection
5283c87 feat: construct and convert structured errors
1838ad8 fix: preserve error families through aliases
ad5b6a7 feat: check fallible flow steps
836a477 feat: require explicit Result handling
c71cbf2 fix: allow explicit Results in final branches
f57332c fix: reject discarded Results in loops
991eb90 feat: add application error reporters
b308214 feat: expose redacted host error reports
cf1b560 fix: bound exported error cause chains
bbec031 feat: support error syntax in editor grammars
1a912b4 feat: highlight error handling in Zed
5a7d2bd feat: support deliberate console exit statuses
fa3de42 feat: add executable error recovery example
3c194e7 feat: add executable console exit example
99b3be5 feat: highlight console exit
7ca9b30 docs: specify checked error recovery and reporting
2af6b56 fix: preserve structured file failure reasons
9ceddad feat: add console failure with explicit stderr routing
00a6f44 docs: document checked recovery and reporting
66decae feat: check explicit binding type annotations
65b05f4 feat: add nominal record values
fe86fb6 feat: persist record definitions in REPL sessions
a3aab94 feat: support qualified module record types
e50425a docs: specify records and collection semantics
6a48630 feat: add typed list literals and contextual types
633cc38 feat: add checked list access and mutation
2783224 feat: add checked rectangular arrays
61d3bcd feat: iterate lists and arrays safely
ee18b29 feat: highlight records and collections in editors
6147e6e chore: pin updated zed grammar
6547f5d docs: add executable records and collections examples
bfc8c32 fix: preserve error families across collection helpers
eff53ab fix: expose array failure details
c415297 test: cover collection guard unwinding
51c7a7a test: cover rich value editor highlighting
fe31f74 fix: clarify error flow restriction
ce67a32 docs: clarify error flow limitations
8100d67 docs: explain records collections and iteration
7b28354 docs: update getting started for rich values
e7e501c fix: make documentation navigation easier to find
7a696e2 docs: guide readers through a first program
87da402 fix: size documentation headings for reading
ae0b6f5 fix: preserve getting started guide links
96b03a1 docs: order and clarify runnable examples
65c4cba feat: display all example source inline
a917c89 fix: show example code first on mobile
af65138 docs: use canonical example commands
bcd686f refactor: isolate named type registration
7ed384b docs: simplify the error recovery example
2932148 feat: add checked choice matching
b999d57 feat: highlight choice and match syntax
0e42a38 chore: pin choice-aware zed grammar
f5c1a99 docs: add an executable choice example
e1ce830 feat: infer fallible word effects
4327d70 fix: parse nested result type arguments
fbd896c feat: infer generic choice type arguments
b2b229a feat: parse generic choices in editor grammar
74b63b1 chore: pin generic choice grammar
3d0d811 feat: pass extra arguments through flows
6aca1b1 docs: demonstrate generic choices
0661b90 fix: bound inferred generic type depth
e5a13f6 feat: add builtin option choices
2d6bf7b feat: add ordered maps and optional lookup
32063a9 feat: support map syntax in editors
484cf55 chore: pin map-aware zed grammar
a2d9c9f docs: add an executable map example
52cc207 fix: reserve map type parameters
a57a64b docs: record checked type and map semantics
7ae6bd9 docs: clarify collection size diagnostic
22adf2c test: verify map assignment evaluation order
3b03355 docs: update map implementation status
0096669 feat: allow plain indexed collection writes
7cb1821 test: cover checked write guard recovery
d06d9fe docs: describe optional checked writes
a10b23f docs: simplify array setup example
22dc1bf docs: explain choices maps and error inference
839de07 docs: simplify grid iteration setup
2a1a58d test: cover indexed write propagation and origin
bd6519f feat: add typed JSON with copied defaults
cd1eed3 docs: add executable typed JSON example
4ef6173 feat: highlight JSON operations in editors
8b1f4bd docs: explain typed JSON defaults
66f8c5d fix: count JSON nesting by wire structure
09baf4b feat: add transparent type aliases
c5a7c9c feat: parse type aliases in editors
1c57453 chore: update Zed grammar revision
92fc7e8 feat: support record field defaults
6753774 feat: parse record defaults in editors
b925600 chore: update Zed grammar for defaults
ad81e53 fix: bound expanded alias type nesting
1948daa fix: clarify record constructor arity
7525415 test: cover record default evaluation order
fdd3bd4 fix: bound imported alias expansion
2ba1e31 docs: explain aliases and record defaults
83177a4 test: cover handled failures in record defaults
9713386 feat: resolve civil time transitions
d4eec27 feat: make collection reads plain by default
6290b85 feat: make collection growth plain by default
8edf1ef fix: bound recursive record defaults
b72508a docs: simplify collection examples and guidance
e48f338 feat: add date time and duration values
2e18f56 feat: add instants and named time zones
f6fc1ad feat: add algorithm examples and grouped corpus
aa0e292 fix: preserve overlap instant for zero calendar days
146e0ac feat: highlight temporal operations in editors
e8813de feat: include temporal component diagnostics
ce2b72c docs: add temporal guide and executable example
d865c5b fix: prevent choice variants shadowing builtins
f68800a docs: record rich values and error guidance
a41883f feat: streamline algorithm examples with flows
51015d5 docs: clarify checked collection operations
3789631 docs: describe Life flow composition
0916260 refactor: simplify Life initialization and generations
8f98dec feat: add editable REPL autoindent
2c5d5a4 fix: preserve pasted whitespace in REPL
5fc99c9 fix: hint print call syntax
1fb2b7f test: verify queued REPL input reaches program
517b923 fix: hint parentheses for bare print
730cd02 fix: harden REPL control keys and long input
f6bf797 test: preserve piped REPL output
a740f20 docs: explain interactive REPL editing
2cfaf37 feat: add ordinary REPL bye word
a676f1e docs: describe the REPL bye word
123fd34 feat: support block values in bindings
9c84aea feat: allow multiline delimited expressions
72ac9f5 docs: explain block-valued bindings
2d065d9 fix: restore logical indentation after multiline input
bf9c0c2 feat: support named record arguments
4aeea43 feat: support new syntax in editor grammar
9f2c4e0 chore: update Zed grammar revision
1a15951 test: cover multiline range and checked writes
9e4865e fix: run multiline input completed at EOF
8c29e8a docs: explain multiline and named constructors
15071eb fix: suggest calls for bare word names
c9f48b1 fix: avoid identical name suggestions
eeb7243 refactor: add bounded type constraints
749144f refactor: expose effect analysis without mutation
4b02484 feat: infer word return types
5111df2 feat: highlight inferred word returns
359e78e chore: update Zed grammar revision
b4cc92c test: check inference alongside example corpus
7dc3c8f test: protect private inferred module types
3e7f8b0 fix: preserve unknown name inference diagnostics
144531a fix: preserve concrete operand diagnostics
66a8500 docs: explain inferred word returns
dc86652 fix: preserve concrete argument diagnostics
8cb7aec fix: preserve condition diagnostics during inference
3f5ef0f feat: infer word parameter types
5596571 feat: highlight inferred parameters
aa11188 chore: pin parameter grammar for Zed
c2cdd18 test: infer unused reporter parameter
51dd184 docs: specify calls and partial application
247d656 test: cover deferred parameter constraints
19677b0 test: check parameter inference across examples
5783ec7 fix: infer caller-owned nominal types
b1c9117 test: infer parameters through choice matches
be9f626 docs: explain inferred parameter types
fa99777 feat: add word parameter defaults
5250b36 feat: highlight word parameter defaults
08677c3 chore: pin default parameter grammar
1dcfcf6 test: infer imported parameters from defaults
42eecd0 docs: add word defaults example
50330dc docs: explain word parameter defaults
0977786 feat: support simple word calls
82e3847 feat: highlight simple word calls
529dd5e chore: pin simple call grammar
f80e317 fix: retain editor index and subtraction parsing
b079a02 chore: pin editor parsing fix
fe15339 docs: add simple calls example
5e3d957 refactor: parse nested function type shapes
50101a8 docs: explain simple word calls
1c0d228 test: verify bare bye restores terminal
7d6a808 fix: preserve bare call try boundaries
5f99f21 fix: lift bare call try across unary arguments
7f4ec7a docs: clarify bare call try boundaries
51cc2b0 feat: recognize checked function types
235c36b feat: add explicit user-word partials
e62a045 docs: add explicit partials example
2d7ad45 docs: order examples from basics to partials
d63f27d docs: explain explicit user-word partials
ae2dd92 feat: specialize partials and infer callable effects
db87625 fix: bound nested function types
bb20ade test: reject mixed callable error families
91470cb feat: highlight function type annotations
5e8076f chore: pin callable type grammar
759d042 docs: clarify non-returning partial boundary
3d3365e docs: demonstrate builtin and fallible partials
b1a2c2c fix: infer overloaded partials from all uses
3b20566 fix: trace default and captured arguments safely
65d55cc fix: preserve deferred callable word context
30e6307 fix: report partial invocation locations
5e06461 test: specialize map partials from context
4227cf2 test: bound recursive partial dispatch
67a9cff fix: resolve callable before evaluating arguments
0b21a0c docs: cover builtin and fallible partials
```
