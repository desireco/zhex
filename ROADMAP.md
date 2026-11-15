# Zhex roadmap

Build a readable language from small, composable words. The Go interpreter lets us try the syntax and settle behavior before choosing a compiled implementation.

## Done

The first language milestone runs real programs with checked types and console input and output.

- Interpreter: typed words, inferred locals, conditions, bounded loops, and checked flows.
- Working examples: composition, console input, FizzBuzz, Fibonacci, factorial, palindrome checking, a recursive GCD, and a times table.
- Executable specification: tests cover syntax, types, scope, arithmetic boundaries, Unicode, and failure paths.
- Getting started: documentation, a public examples website, just recipes for development, and local highlighting packages for Vim/Neovim, VS Code, and Zed.
- Website: Astro-powered topic documentation and an editorial redesign with a new logo, syntax-highlighted examples, and a published design system with a concept archive.
- Licensing: the repository is public and published under the MIT license.
- Editors: syntax highlighting for Vim/Neovim, VS Code, and Zed, with installation and usage documented on the website's docs page.
- Agents: AGENTS.md, an llms.txt site index, and agent-readable examples make Zhex easy for humans to review and AI agents to write.
- Errors: built-in diagnostics point at the source, carry stable cataloged codes, and can report as JSON; user words declare success types and infer a single escaping Result family through `try`, then callers propagate with `try` or recover lazily with `catch`. Multiple uncaught families are rejected; explicit `Result<T, E>` signatures remain supported. Results cannot be silently discarded. Structured Error values expose code, origin, details, and causes; a named reporter can observe explicit handled reports and each unhandled terminal failure without external delivery by default. `console.exit` provides deliberate statuses from 0 to 255; `console.fail` writes a message to stderr and exits with status 1. Both close active `file.stream` loops on termination.
- Input: `read-line`, `text.to-int`, file, buffer, and argument reads return explicit Results. Callers choose a fallback with `catch`, propagate through compatible fallible words with `try`, or let the program boundary report an unhandled failure.
- Insight: `zhex run --trace` streams JSONL step events — word calls, bindings, flow steps, returns — so people and agents can watch a program run; the event vocabulary is documented in llms.txt.
- REPL: `zhex repl` keeps words and values across entries, enters multiline blocks and delimited expressions with editable terminal auto-indentation, four-space Tab and bracketed paste support, echoes expression values, redefines words with a note, lists signatures with `words`, recovers from errors, and traces with `--trace`.
- Vocabulary: rich text work ships — sixteen `text` words, with literal split, join, and prefix/suffix checks added by the first standard-library story (#57), `Buffer` and `File` types, guaranteed-close `file.stream` loops, whole-file reads through `file.read`, checked report output through `file.write` with the first built-in trailing constant default (#58), and program arguments via `console.arg`.
- Vocabulary: collection composition ships — `list.map`, `list.filter`, and `list.fold` run ordinary function-value callbacks (explicit or word-name partials, no lambda syntax) once per element left to right, with fresh outer lists, shared elements, an unchanged source, and the traversal guard held throughout (#60), and `list.find`, `list.any`, and `list.all` short-circuit over the same callback contract, with `find` returning `Option.Some` or `Option.None` (#61); Result-returning callbacks and Result fold accumulators are rejected, and the registered `score-report` example demonstrates all six words.
- Vocabulary: deterministic ordering ships — `list.sort` returns a stable ascending copy of `List<Int>`/`List<String>` values in code-point string order, and `list.sort-by` ranks any list by an Int or String key callback evaluated once per element in source order under the traversal guard (#62); both return fresh outer lists with shared elements and unchanged sources, equal keys keep source order, and the registered `sorting` example demonstrates both words.
- Vocabulary: file inspection ships — `file.list(folder)` returns one level of a folder's bare entry names, dot entries included, as a fresh sorted `List<String>` inside a `Result` under the shared million-element limit, `file.exists(path)` answers existence as a plain `Bool` where a missing path is `false` (postfix `try`/`catch` selects the checked `Result<Bool, FileError>` form), and `file.size(path)` and `file.free-space(path)` report a regular file's bytes and the bytes available on a path's volume inside Results (#73); failures carry the stable codes `file-list`, `file-exists`, `file-size`, and `file-free-space`, and the executable example is deferred until #75's `file.make-folder` makes a deterministic one possible.
- Vocabulary: lexical path words ship — `file.join(base, child)` concatenates with the native separator and cleans, with an absolute child replacing the base (`file.join("a/b", "/c")` → `/c`) so untrusted absolute components never nest under a base; `file.base(path)`, `file.extension(path)`, and `file.directory-of(path)` decompose a path into its final element, final-dot suffix (dot included, `.bashrc` for the dotfile), and everything but the final element (#74); all four are pure `String` words with no filesystem access, and the registered `paths` example demonstrates them.
- Rich values: explicit binding annotations, nominal records with named constructor arguments and optional field defaults, block-valued bindings, transparent type aliases, and one-parameter choices with exhaustive matching, homogeneous shared `List<T>` and shaped `Array<T>` collections, inferred `Map<K, V>` with built-in `Option<V>` lookup, plain list/array reads and mutations with optional checked forms, insertion-ordered map operations, and collection traversal with bindings and mutation guards. Typed JSON decoding with explicit record defaults and encoding for the supported value subset is also available. Dates, times, durations, instants, and named zones support calendar and elapsed-time operations.
- Text utility: `text-stats` reads a file two ways — whole-buffer counts and a streamed uppercased echo — with program arguments, streamed lines, and guaranteed close, all through the new vocabulary (#28) — and becomes the workload for measurements (#4).
- Measurements: the interpreter runs text-stats at 8-9 ms per pass with stable allocation profiles by mode (docs/measurements.md); readability review recommends a line-filter tool as the next application, with number-to-text ceremony flagged for a future language story.
- Modules: `use math` loads sibling files, words are importable unless underscore-private, calls are qualified, cycles and reserved names diagnose with paths, and the REPL adopts modules per session — with a two-file corpus example and docs sections to match.

## Next

Grow the language's reach: a dedicated domain and funding. This is the proposed order; each step needs working examples and tests.

- Standard library: the read-API rename and folder research are done (#68/#69), and file inspection (#73) and lexical path words (#74) shipped as the first two folder-operations stories; destructive operations are last in the approved order (S10, #75), with list `select` (#70), editor keyword sync (#71), and the document and Markdown research (#66/#67) queued in Todo. #57's literal text operations, #58's `file.write`, #59's edit-distance simplification, #60/#61's list callback words, and #62's list sorting shipped. The six-language research and dependency order live in `docs/standard-library-plan.md` (#30); proposed operations are not shipped.
- Domain: buy a dedicated domain for the language and point it at the published site.
- Funding: enable GitHub Sponsors or Open Collective with a sponsor section on the site, and pursue the researched grant and visibility channels (#26).

## Future

These are directions to explore. Their order and implementation remain open, and there are no delivery dates yet.

- Dogfooding: investigate a Zhex-written REPL front end over the Go session engine (#65), beginning with a bounded prototype and explicit host/input boundaries; full interpreter self-hosting is not in scope.
- Rich values: generic records and words, and user-defined error types with readable operations and formatting.
- Compiled execution: evaluate a native backend, including C as a possible target, and measure composition, text processing, and allocation behavior.
- WebAssembly code generation: compile Zhex programs to WASM, starting with browser execution and a small host interface for input and output.
- Application libraries: grow through small CLI, HTML-processing, web-server, game, and database experiments.
- Development tools: formatting, language-server support, and published editor packages as the syntax settles.
- Development insight: record-and-replay runs, debug-adapter support, and agent tools, so people and AI agents can see and debug what a program is doing (see docs/diagnostics-plan.md).
- Module growth: exposure lists, import renames, subfolder paths, and computed module constants, composing on the simple use-and-underscore design when each earns its place.
