# Diagnostics and development-insight foundation

Zhex is young: the checker, the diagnostic type, and the run loop are small and every error path already funnels through one place. That is exactly when a foundation is cheap to set and expensive to skip. This document commits Zhex to a diagnostic contract, records what Elm (and the systems in the [research report](error-reporting-research.md)) teach us, and lays out a phased plan — including how development-mode insight and AI-assisted debugging get built in later without rework. Input to [#17](https://github.com/desireco/zhex/issues/17); reviewed twice before commit.

## Part 1 — What Elm teaches (the golden parts)

Elm's error messages are the reference the industry converged on — Rust's error renaissance is openly credited to them. Four practices matter to Zhex:

1. **Errors are UX, with a teacher's voice.** Errors have human categories (`TYPE MISMATCH`, `NAMING ERROR`, `SYNTAX PROBLEM`), explain what the compiler expected vs. got, narrate where values came from ("this `value` is a `String` because it comes from..."), and end with a `Hint` that names a fix — never speculation ([Compiler Errors for Humans](https://elm-lang.org/news/compiler-errors-for-humans), [Compilers as Assistants](https://elm-lang.org/news/compilers-as-assistants)).
2. **A shipped catalog.** The compiler repo carries `hints/*.md` — one markdown file per error class (e.g. [`infinite-type.md`](https://github.com/elm/compiler/blob/master/hints/infinite-type.md)) — so every error can point to a durable, linkable explanation instead of growing an essay inline ([hints/ directory](https://github.com/elm/compiler/tree/master/hints)).
3. **Machine-readable from day one.** `--report=json` shipped *with* the human redesign, not after it. The same diagnostic exists as prose and as data; editor plugins and build tools never scraped text.
4. **Runtime insight exploits the language's guarantees.** Elm's time-travel debugger records the message sequence and replays it through a pure `update`, so scrubbing history is just re-running a function ([Time Travel made Easy](https://elm-lang.org/news/time-travel-made-easy)). `Debug.log` is the cheap tap underneath: label a value, print it, return it unchanged. The lesson is not "build Elm's debugger" — it is *find what your language already guarantees, and make that observable*. Record-and-deterministic-replay is the same principle in the general case ([Replay docs](https://docs.replay.io/reference/time-travel/how-does-time-travel-work)).

Zhex's equivalent guarantees: the checker already knows every name, word signature, and type before anything runs; and execution is deterministic given the program and its stdin. Both are observable for free if we build the taps now.

## Part 2 — The Zhex diagnostic contract

Every diagnostic, present and future, obeys these rules. Breaking one is a bug.

1. **Stable first line, forever.** `file:line:column: message` — scripts, tests, and agents parse it without change.
2. **Detail is additive.** Excerpt + caret line, then an optional `hint:` line — zero or one hint; multiple hints per diagnostic are a redesign, not a PR. Nothing may move the first line.
3. **One diagnostic, two renderings.** A single `diagnostic` value drives the human renderer and the machine emitter; prose and data can never disagree (Elixir 1.15 and rustc's rule).
4. **Every diagnostic carries a stable code** (`unknown-name`, `result-unwrapping`, …) once Part 5's P1 lands. Codes are kebab-case, never renamed or reused, and documented one anchored section per code on the catalog page.
5. **Hints are earned, not guessed.** A hint ships only when it names a concrete fix the compiler can vouch for (did-you-mean with a distance threshold, the `try` rule, the hyphen rule). No "maybe try checking X" — speculation is worse than silence.
6. **Narrate provenance when it is known.** Type mismatches should say where the value came from when the checker knows: "operand is `Result<Int, InputError>` because `m` comes from `console.read-int`". Provenance turns a complaint into a lesson.
7. **The catalog is part of the tool.** Codes link to a docs page per code (Elm's `hints/` + Svelte's warning docs pattern); llms.txt and AGENTS.md point at the catalog so agents can look up guidance.
8. **Errors never hide behind enhancement.** Suggestions, carets, colors, JSON — all additive and optional. This activates with the first enhancement that changes rendering: any such enhancement ships with its off switch in the same phase (colors will honor `NO_COLOR`). The bare diagnostic is always available.

## Part 3 — Architecture (small on purpose)

The current shape already supports most of this: `diagnostic` (`internal/lang/syntax.go`) is produced at every raise site via `errorAt`, hints attach via `withHint`, and `annotate` is the single choke point that adds the excerpt. The plan:

- `diagnostic` grows `code string` and `severity string` fields; every raise site passes one of the registered codes through `errorAt`'s `Code` parameter — the as-built evolution of the coded-constructor idea, with the same no-drift guarantee delivered by the bijection test (codes ↔ catalog) plus a per-code pairing test that pins which code each raise site produces. The code list is exported from `internal/lang` so tests and the site can enumerate it. The only severity today is `error`; a second value arrives with the first warning the language grows.
- Cardinality is pinned: **one diagnostic per run, zero-or-one hint** — the JSON emitter emits a single object, never an array or a stream. The checker collecting multiple diagnostics (as Elixir 1.16 does) stays a listed extension in P1's out-of-scope guard until it gets its own phase. The exported `Run`/`Program.Run` signatures keep returning `error`, rendered from that one diagnostic, so `cmd/zhex` and the site builder keep working unchanged.
- Renderers: `renderText(d)` (today's output) and `renderJSON(d)` (`{code, severity, file, line, column, message, hint}`), chosen by `--report=text|json` on `zhex run` — Elm's flag, spelled the same. This plan is the single spec for the channel; the research report's `--json` sketch and #17's earlier wording are superseded by it.
- The catalog prose lives in `internal/lang` (markdown files, Go-embedded beside the coded constructors), and the website's `errors.html` renders it. The completeness guarantee lives where the codes live: a Go test in `internal/lang` fails if a code lacks a catalog entry — that keeps the contract intact even when the site moves to Astro (a standing roadmap item), since the site merely renders.

## Part 4 — Development-mode insight (the possibility we open)

Goal: a person — or an agent — can answer "what is this program doing and why" without adding print statements. Staged so each phase is independently useful:

- **Phase A — `--trace`:** `zhex run file.zhex --trace` emits JSONL step events (word call with args, binding write with value, flow step, word return) to stderr. The interpreter already walks exactly these steps; emitting them is a tap, not a redesign: `runtime` grows a trace writer, `Program` gains a run variant that takes one, and the default run path stays untouched. Agents grep it; humans pretty-print it. This is `Debug.log` generalized, without touching programs.
- **Phase B — record & replay:** capture the stdin a run consumed into a sidecar file; `zhex run --replay=…` re-executes deterministically. Because evaluation is pure given input, replay is exact — Elm's record-the-messages trick mapped onto Zhex's stdin. A failing run becomes a reproducible artifact an agent can step through repeatedly.
- **Phase C — breakpoints and stepping:** pause/step/inspect events shaped like the [Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/), so editors and any DAP client (agents included) drive the same run loop that `--trace` and `--replay` already instrument.
- **Phase D — agent surface:** an MCP-style server exposing check/run/trace/replay as tools, so an agent can debug a Zhex program the way it uses a terminal today. Only becomes real once A–C exist; no custom protocol of our own.

Guardrails: all of this is dev-mode-only flags; default runs keep today's byte-identical output.

## Part 5 — Phased plan and ticket mapping

| Phase | Work | Status / ticket |
|---|---|---|
| P0 | Carets, did-you-mean, earned hints | Done (`5bf7afc`, [#18](https://github.com/desireco/zhex/issues/18)) |
| P1 | Codes + coded constructors, `--report=json`, `errors.html` catalog, llms.txt link. Out of scope: multi-diagnostic collection, any severity beyond `error`, colors. | Done ([#17](https://github.com/desireco/zhex/issues/17)) |
| P2 | Spans (caret covers the whole expression) + provenance narration in type mismatches. Out of scope: anything runtime. | ticket when P1 lands |
| P3 | `--trace` JSONL step events | ticket after P2 |
| P4 | Record & replay, then DAP-shaped stepping; MCP surface last | on the roadmap as Future (Development insight) until a phase is picked up |

**Guidance for every future PR touching diagnostics** — the checklist a reviewer applies:

1. First line unchanged? Detail additive?
2. (after P1) New error site has a code, a coded constructor, and a catalog section?
3. Does the hint name a fix the compiler can vouch for? If not, no hint.
4. (after P2) Can the checker narrate provenance here? If yes, it should.
5. Test asserts message *and* hint content (tests are the language definition).
6. (after P1) JSON emitter and text renderer agree, because they render one value.
7. Docs that record truth updated: `errors.html`, AGENTS.md, llms.txt, ROADMAP.md, the project board, and this plan's status table.
