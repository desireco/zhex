# Error reporting and LLM guidance research

How mature languages make diagnostics actionable for humans and machines, and what Zhex should adopt. Written for [#18](https://github.com/desireco/zhex/issues/18); the full feature set lands in [#17](https://github.com/desireco/zhex/issues/17).

## What each language does

### Elixir — the compiler teaches the language

Elixir treats diagnostics as a learning surface, not a failure report.

- **Did-you-mean since 1.5 (2017).** Converting warnings into errors unlocked suggestions: *"By converting it to an error, we can also add a 'did you mean?' feature"* ([elixir-lang/elixir#5461](https://github.com/elixir-lang/elixir/issues/5461)). Undefined function errors list close matches (`Did you mean one of: parse/2, parse/3`), powered by `String.jaro_distance/2` ([String docs](https://hexdocs.pm/elixir/1.12/String.html)).
- **Structured diagnostics API in 1.15 (2023).** The compiler emits diagnostics as data (file, line, column, severity, message) so editors and tools consume them without scraping text ([Elixir v1.15 release](https://elixir-lang.org/blog/2023/06/19/elixir-v1-15-0-released)).
- **Fancy rendering in 1.16 (2023).** Code snippets with the offending line, multiple diagnostics per compile, and color — designed in the open in [elixir-lang/elixir#12644](https://github.com/elixir-lang/elixir/issues/12644) ([release post](https://elixir-lang.org/blog/2023/12/22/elixir-v1-16-0-released)).

The pattern: **the error carries the fix**, and the same diagnostic exists as data (for tools) and as a rendered snippet (for humans).

### Ruby — suggestions and underlines as default gems

- **[did_you_mean](https://github.com/ruby/did_you_mean)** hooks `NameError`/`NoMethodError`, computes the misspelled name, and appends `Did you mean? …` using Levenshtein-style similarity with length-aware thresholds (edit distance 1 for short names, 2 for longer).
- **[error_highlight](https://github.com/ruby/error_highlight)** (default since Ruby 3.1) maps the failing AST node to a line/column span and prints the source line underlined with `~~~` under exactly the receiver expression that failed ([walkthrough](https://blog.saeloun.com/2022/02/16/ruby-3-1-adds-error-highlighting), [RubyKaigi 2022 talk](https://rubykaigi.org/2022/presentations/mametter.html)).
- Both are disable-able (`--disable-did_you_mean`) — the enhancement never hides the underlying error.

### Swift (and the Elm benchmark) — fixes, not just findings

Swift's diagnostics pair the mismatch with **fix-its**: concrete replacements the user can accept, plus candidate listings for ambiguous calls. Swift 6.2 continued this by making type errors hint the repair instead of reporting "ambiguous overload". Community discussions routinely name **Elm the gold standard** — its type mismatch errors explain *why* the types diverged and suggest the fix ([thoughtbot on Elm's errors](https://thoughtbot.com/blog/understanding-elms-type-mismatch-error), [Swift Forums](https://forums.swift.org/t/replacing-the-type-checker/79518)). Notably, forum participants mention developers *pasting Swift errors into AI assistants* to interpret them — evidence that compilers without actionable hints push interpretation work onto the model.

### rustc, TypeScript, Svelte — machines are first-class readers

- **rustc** ships `--error-format=json`: one JSON object per diagnostic with `message`, `code`, `level`, `spans`, `rendered` — the channel `rust-analyzer` and CI tooling parse ([rustc dev guide](https://rustc-dev-guide.rustc-lang.org/diagnostics.html); the structured-diagnostics [pre-RFC](https://internals.rust-lang.org/t/pre-rfc-fully-structured-rustc-diagnostics-towards-l10n/4576) and [cargo#11018](https://github.com/rust-lang/cargo/issues/11018) cover the design). The human rendering and the JSON carry the same content.
- **TypeScript** tags every diagnostic with a stable code (`TS2345`) with a searchable page per code.
- **Svelte** gives every compile warning/error a stable `code`, documents all codes on one page, and tools filter by code ([compiler warnings docs](https://svelte.dev/docs/svelte/compiler-warnings), [compiler API](https://svelte.dev/docs/svelte/svelte-compiler)).

### The 2025 agent-era convergence

Tooling built for LLM agents now assumes the rustc pattern: **stable codes + structured JSON + exact positions + repair hints**, so an agent can self-correct instead of retrying on prose. See [Why AI coding agents love type errors](https://encore.dev/blog/type-errors-agents) (compile-time, line-pointing, unambiguous errors are the strongest self-correction signal) and [Zero's JSON diagnostics](https://coddy.tech/docs/zero/json-diagnostics) (stable codes and structured repair plans designed for LLM consumption).

## The distilled practice

Every studied system converges on the same five moves:

1. **Point at the code** — print the failing source line with a caret/underline at the exact column (Elixir 1.16, Ruby error_highlight).
2. **Suggest the fix** — did-you-mean for names (Elixir jaro, Ruby Levenshtein), fix-its for types (Swift, Elm). The hint is additive; the machine-readable first line never changes.
3. **Make the diagnostic data** — the same diagnostic available as structured output for editors/tools/agents (Elixir 1.15 API, rustc JSON).
4. **Give every diagnostic a stable code** — referenceable in docs, filterable by tools (rustc `E0308`, TS `TS2345`, Svelte `a11y_*`).
5. **Teach, don't scold** — wording assumes the reader is learning the language; the two most common beginner mistakes get their explanation built into the error (Elixir's philosophy).

## What Zhex adopts

| Practice | Verdict | Reason |
|---|---|---|
| Source line + caret in every diagnostic | **Adopt now** | One render point (`diagnostic.Error`); columns are already rune-accurate. |
| Did-you-mean for unknown names | **Adopt now** | Small candidate set (locals + words + host words); Levenshtein ≤ length-aware threshold, Ruby-style. |
| Built-in hints for the two documented gotchas | **Adopt now** | Hyphenated names and `Result` without `try` are already the two fixes AGENTS.md teaches — move them into the tool. |
| Stable first line `file:line:column: message` | **Keep** | Existing parsers (agents included) keep working; hints/excerpts are additive lines. |
| Stable diagnostic codes | **Adapt (next, #17)** | Adopt when #17 lands; each diagnostic gets a `code` field and one docs section per code. |
| Machine-readable channel (`--json`) | **Adapt (next, #17)** | Emit `{code, file, line, column, message, hint}` per diagnostic; same fields the renderer uses. |
| Fix-its / machine-applicable patches | **Skip for now** | Zhex's hints name exact textual fixes (`a - b`, postfix `try`) which agents apply trivially; structured patches can come with the language-server roadmap item. |

## Applied slice (this ticket)

Implemented alongside this report:

- Every diagnostic through `zhex run` now prints the source line with a caret at the column, then an optional `hint:` line.
- `unknown name "a-b"` grows the hint *"`a-b` is one name — hyphens join a single identifier; for subtraction write `a - b` with spaces"* (the AGENTS.md gotcha, moved into the tool).
- Unknown names and words suggest the closest known name: `did you mean "fibonacci"?`.
- Any `Result<...>` reaching a plain-value position (operators, conditions, `print`, arguments) grows the hint to add postfix `try`.

Deferred to #17 with this research as input: stable codes, the `--json` channel, per-code docs.
