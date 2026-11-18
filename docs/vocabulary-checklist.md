# Vocabulary-change checklist

A built-in word is stated in more places than the interpreter: editors, the language contract, the website, README, and the example corpus all restate the vocabulary. This document is the canonical list of those surfaces for three changes — a **new word**, a **signature change**, and a **rename or removal**. Stories #57–#68 each re-derived the list ad hoc and twice missed surfaces (the editor word lists drifted, and llms.txt's Example-sources line was forgotten once); do not re-derive it again.

Tests are the language definition: write the failing behavioral test first (see [tests.md](tests.md)), then the implementation, then every surface the change touches.

## Surfaces

### Interpreter (`internal/lang`)

- [check.go](../internal/lang/check.go) — the `hostWords` registration (also the did-you-mean suggestion pool), the call-checker typing switch, `builtinTrailingDefaults` for constant trailing defaults, and the loop-only cases for `range` and `file.stream`.
- [host.go](../internal/lang/host.go) — the `builtin` evaluation switch.
- [infer_returns.go](../internal/lang/infer_returns.go) — return-type inference for built-in calls.
- [effects.go](../internal/lang/effects.go) and [effect_constraints.go](../internal/lang/effect_constraints.go) — escaping Result-family inference; a new family changes propagation rules for every caller.
- [syntax.go](../internal/lang/syntax.go) — the result-type constants and `resultParts`/`resultFamily`.
- [collections.go](../internal/lang/collections.go) — `isCheckedCollectionOperation`, when the word joins the plain-write/checked-Result mechanism.
- [parser.go](../internal/lang/parser.go) — loop-only keyword forms, parsed only inside `for` headers.
- [check_functions.go](../internal/lang/check_functions.go) — partial-application rules: constructors, reporter registration, and non-returning words are excluded; overloaded built-ins need an annotation.
- [codes.go](../internal/lang/codes.go), [codes_test.go](../internal/lang/codes_test.go), and [catalog.md](../internal/lang/catalog.md) — only when the change adds or retires a diagnostic code. `codes_test.go` fails when the constants and catalog drift; the site's errors page is generated from `catalog.md`, so it needs no separate edit.

### Editors (`editors/`, details in [editors.md](editors.md))

- `editors/vim/syntax/zhex.vim` — the `zhexBuiltin` keyword line.
- `editors/vscode/syntaxes/zhex.tmLanguage.json` — the built-in match pattern.
- `editors/tree-sitter-zhex/grammar.js` — keyword forms in the `for` header. After editing it, run `tree-sitter generate --abi 14`, commit the generated `src/` files, update `test/corpus/syntax.txt`, and keep `queries/highlights.scm` identical to `editors/zed/languages/zhex/highlights.scm`.
- `editors/zed/extension.toml` — the grammar revision pin moves with every grammar change; re-run `just zed-prepare`.
- Drift prevention is [#71](https://github.com/desireco/zhex/issues/71)'s job — a generator plus a pinning test, so a story cannot forget editors again. Until it lands, update every list by hand; the `editors.yml` workflow verifies the result.

### Truth surfaces

- `AGENTS.md` — the standard-operations line; the canonical-patterns sentence when an example registers; the reserved-module list when a new namespace arrives.
- [initial-language.md](initial-language.md) — the contract; every behavior change lands here.
- A reserved name beyond the documented ones (the `Option` constructors, and `main` as the zero-argument entry word of #134) is a contract addition, not a new word: it lands on the same truth surfaces with no `hostWords` entry, no editor keyword, and no example registration — and AGENTS.md plus initial-language.md must say both where the name is special (the entry file's top level) and where it stays ordinary (beside statements, in modules, in the REPL).
- [standard-library-plan.md](standard-library-plan.md) — the inventory table row and the registered-names count sentence. Re-count the entries in `hostWords` and write that number; never adjust the previous number by arithmetic.
- [syntax-examples.md](syntax-examples.md), [getting-started.md](getting-started.md), `README.md` (its `What it looks like` program must run), and [tests.md](tests.md) (test-catalog rows and the example-count sentence).
- `site/src/content/docs/<namespace>.md` and `site/src/content/docs/standard-library.md`; `site/llms.txt` (the namespace word list, and the Example-sources line when an example registers).
- `ROADMAP.md` — the story's line, keeping the Done/Next/Future shape the website generator validates.

### Example registration (when the change ships or updates an example)

- `examples/<name>.zhex`.
- `internal/lang/examples_test.go` — the `TestExamples` row with arguments and expected output.
- `tools/site-data/main.go` — the `exampleSpecs` entry; the website executes every registered example at build time, so a broken example fails the build.
- `tools/site-data/main_test.go` — the example-count pin.
- The mirrors that restate the corpus: the `README.md` examples table, [syntax-examples.md](syntax-examples.md), the [tests.md](tests.md) count sentence, the `site/llms.txt` Example-sources line, and the `AGENTS.md` canonical-patterns sentence.

## New word

- Failing behavioral tests in `internal/lang` first — the namespace's `*_test.go` or a new file — plus the `TestExamples` row when an example ships.
- `check.go`: the `hostWords` entry, the typing case, `builtinTrailingDefaults` for constant defaults, the loop-only cases when the word is loop-only, and the reserved-name check when the name could collide with a user word or reserved module.
- `host.go` evaluation; `infer_returns.go`; `effects.go`/`effect_constraints.go` with the `syntax.go` result constants when the word is fallible; `collections.go` when it joins the checked mechanism; `parser.go` when it introduces a loop-only form; `check_functions.go` partial rules; `codes.go` + `catalog.md` for each new diagnostic code.
- Every editor surface above, including the tree-sitter regeneration and the Zed revision pin.
- Every truth surface above, with the `hostWords` count re-derived in [standard-library-plan.md](standard-library-plan.md).
- The example-registration mirror set when an example demonstrates the word.
- Gates: gofmt on changed Go files, `go vet ./...`, `just test` (its site build needs `npm` from the local node install on `PATH`), and the `editors.yml` workflow.

## Signature change

- Failing tests first: every changed parameter, default, and return result, and each call the new signature rejects.
- The interpreter sync points the signature touches: the `check.go` typing case, the `host.go` case, `infer_returns.go`, `effects.go`/`effect_constraints.go` and the `syntax.go` constants when the Result family moves, `collections.go` checked forms, `builtinTrailingDefaults`, `check_functions.go` overload and partial inference, `parser.go` loop-only forms.
- Every caller of the old signature: `examples/*.zhex`, the tests, and the `README.md` `What it looks like` program.
- Every truth surface that states the signature: `AGENTS.md`, [initial-language.md](initial-language.md), [standard-library-plan.md](standard-library-plan.md), [syntax-examples.md](syntax-examples.md), [getting-started.md](getting-started.md), `README.md`, the site namespace page and `standard-library.md`, `site/llms.txt`, [tests.md](tests.md).
- Gates: gofmt, `go vet ./...`, `just test` (the site build executes every example, so stale callers fail the build).

## Rename or removal

The [#68](https://github.com/desireco/zhex/issues/68) precedent: the old names must fail as unknown words with a did-you-mean hint naming the replacement.

- Failing tests that pin the retirement first: the exact `unknown word "old.name"` diagnostic and its `did you mean "new.name"?` hint, in the namespace's test file and `diagnostics_test.go`. Hint voice stays literal and specific — the hint names the replacement, never a metaphor or a gesture.
- Remove the interpreter entries: `hostWords`, the typing and evaluation cases, the `syntax.go` constants, `effects.go`/`effect_constraints.go`, `collections.go`, the `parser.go` loop-only keyword, `check_functions.go`. A diagnostic code is dropped only when it dies with the word — codes are never renamed or reused.
- A stale-name grep over the whole repository — `grep -rn "old.name"` from the repository root — returning zero hits outside the pinning tests and deliberate history notes (the shipped-notes sentences in [standard-library-plan.md](standard-library-plan.md)).
- Every editor surface, including the regenerated tree-sitter `src/` and the moved Zed revision pin.
- Every truth surface, with the `hostWords` count re-derived; the rename recorded in the story's shipped notes the way #68 recorded `file.read`/`file.stream`.
- Gates: gofmt, `go vet ./...`, `just test`, the `editors.yml` workflow, and the stale-name grep quoted in the ship note.

## Gates

- `gofmt` on changed Go files; `go vet ./...`.
- `just test` — `go test ./...` plus the site build; `npm` must resolve from the local node install (`PATH=/home/zhex/.local/share/mise/installs/node/24.20.0/bin:$PATH just test` when the shell lacks it).
- `codes_test.go` and the site-data count pins fail the suite when the catalog or the example corpus drifts.
- The stale-name grep after any rename or removal.
- `ROADMAP.md` keeps its Done/Next/Future shape; the site build validates it.
