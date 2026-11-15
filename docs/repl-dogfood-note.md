# REPL dogfooding design note (#65)

Prototype record for the Zhex-written REPL front end. The frozen owner
decisions (2026-11-14, issue #65) are law; this note documents what was
built inside them, the export format, and every measured divergence.
Nothing here ships: the seam is per-run harness machinery, the driver
lives under `tools/dogfood/`, and the production `zhex repl` is untouched.

## Architecture decision

Option (b): Zhex owns the loop over a primitive seam; Go owns the engine.
Full self-hosting stays excluded — the recursive-type rejection alone
rules it out, and nothing in this experiment argues for revisiting that.

| Concern | Owner | Where |
| --- | --- | --- |
| Session construction, bye preload, writer split, exit propagation | Go | `tools/dogfood/main.go` |
| Prompts, dispatch, buffering, incompleteness, EOF policy, echo/diagnostic routing | Zhex | `tools/dogfood/repl.zhex` |
| Checking, evaluation, transactional session semantics | Go engine | `internal/lang` |

## The seam

`Session.RegisterReplSeam()` (in `internal/lang/repl_seam.go`) registers
seven host-evaluated words plus the `EnterResult` choice into one
session's word/record tables. A `word.host` Go func on the word struct is
evaluated by the runtime instead of a body. The words close over the one
Session; there is no handle type, keeping a later handle non-breaking.
They never join `hostWords`, the diagnostic catalog, editors, site docs,
or llms.txt, and a session that does not register them does not know
them (pinned by test).

- `repl.enter(source: String) -> EnterResult` — one session entry.
  `EnterResult` is `Ran(echo: String)` / `Failed(report: String)` /
  `Exited(status: Int)`, carrying rendered text, not structured fields.
- `repl.words() -> List<String>` — exactly the `words` command's list
  (host words filtered out).
- `repl.incomplete(source: String) -> Bool`,
  `repl.open-delimiter(source: String) -> Bool` — the same compile probe
  the production loop uses, non-executing.
- `repl.diagnose(source: String) -> String` — the probe's rendered
  diagnostic, "" when clean; the piped EOF unclosed-delimiter path prints
  exactly this text.
- `repl.save(path: String) -> Result<Unit, FileError>` — writes
  `Session.Export()`.
- `repl.load(path: String) -> Result<Unit, FileError>` — reads a file
  and Enters it as one transactional entry. A read failure carries a
  FileError; an entry failure keeps its own diagnostic inside the Result
  (the type-level family is FileError; `error.code` reports the entry's
  real code); an exit requested by the loaded file propagates as an exit.

## Engine machinery this needed

- In-flight entry scopes: `Session.active` keeps a running entry's scope
  framed across nested Enters (a driver's `buffer`/`pending` state would
  otherwise be pruned after the first user entry), without joining the
  adopted scope, so user entries still cannot name driver bindings. A
  failed entry pops before its prune, keeping the failed-entry
  frame-release semantics unchanged.
- Adoption merge: a successful entry merges its words, records, modules,
  uses, sources, and scope into the *current* state instead of replacing
  it with check-time copies, so state a nested Enter adopted while the
  outer entry ran survives. With no nesting the behavior is identical to
  the previous assignment.
- Parser source slices: each top-level word/type declaration captures its
  source lines (`word.source`, `recordType.source`), so Export re-emits
  definitions verbatim, including the current body of a redefinition.
- Parser fix: the pseudo-kind lookahead treated a binding *named* `eof`
  as the end-of-file token; `indent`/`dedent`/`newline`/`eof` no longer
  match by name text (pre-existing bug, found by the driver).

## Session export format

`Session.Export() (string, error)` renders one Zhex source file:

```
# zhex session v1        header
use math                 every direct use, in adoption order
<type source slices>     session-declared types, adoption order
<word source slices>     own words, adoption order, current bodies
let b: Buffer = buffer.new()      bindings, sorted by name, always
buffer.add-line(b, "one")         annotated; buffers add one line per
let grid: Array<Int> = array.new([2, 2], 0) try   arrays rebuild with
grid[0, 1] = 5                   fill + indexed writes
var count: Int = 4
error.set-reporter(report)       when a reporter is installed
```

The literal printer covers Int/Bool/String (Go quoting), lists and maps
(empty via the annotation), records as all-named constructors (module
types qualified), choices with payloads, Option, Json trees, arrays, and
temporal values through their constructors — Date/Time via
`date.parse`/`time.parse` (nanosecond-preserving), Duration via
`duration.new`, Instant via `instant.parse`, TimeZone via
`timezone.load`, ZonedDateTime via `zoned.new(date.parse try,
time.parse try, timezone.load try)`. Unserializable bindings — open
Files, function values, Errors, Unit — are skipped with a `# note:
skipped <name>: ...` line in the file. A Buffer or Array nested inside
another value skips the whole binding (no reconstruction path).

Loading Enters the file as one entry: transactional, existing
redefinition rules apply. Known edges, accepted:

- Module drift: `use` lines re-resolve against the loading process's
  working directory; a changed or deleted module file fails the load or
  loads different code.
- The harness preloads `bye`, and the export carries it like any user
  word, so loading into a fresh dogfood process prints
  `redefined bye` once.
- A `ZonedDateTime` inside a DST fall-back overlap reconstructs to the
  later offset (civil + zone is ambiguous there).
- An empty array of a non-primitive inner type has no fill literal and
  skips.

## Driver

`tools/dogfood/repl.zhex` (piped only): a `while not eof` loop over
`read-line(prompt)` — indefinite iteration without recursion — holding
`buffer`, `pending`, `block-pending`, `open-delimiter`, `eof` as
bindings. The state machine is the piped branch of the production loop
(`cmd/zhex/main.go`), with production's `continue` restructured as
if/elif chains: not-pending dispatch (`words` command, empty line,
incompleteness), pending accumulation (blank submits unless a delimiter
is open), and the EOF policy (open-delimiter candidate diagnosed, then
`console.exit(0)`; production's EOF enter branch is unreachable at clean
EOF because #72's read-line returns a partial final line as a successful
read, so the driver submits it through the normal path instead).

## Transcript matrix results

`tools/dogfood/main_test.go` runs both binaries as subprocesses with
stderr sharing stdout's file descriptor, so the merged transcript is the
true write order. 29 scripted cases plus 350 generated entries match
byte-for-byte in write order: bindings, compatible/incompatible
redefinitions, words, use (with module), check errors, runtime errors
with retained effects, session survival, incomplete blocks, open
delimiters, EOF mid-block (discard), EOF unclosed delimiter, queued
program input, bye/console.exit/console.fail, and empty scripts. Exit
statuses match, including `console.exit(999)` — a runtime diagnostic,
after which both REPLs continue to EOF and exit 0.

Measured divergences (all pinned by tests):

1. **Diagnostic stream.** Zhex has no stderr writer, so the driver
   routes `Failed` reports and the EOF unclosed-delimiter diagnostic
   through `print` to stdout; production writes them to stderr. Write
   order and bytes are identical; only the fd differs. `console.fail`
   messages stay on stderr in both (engine-side writer).
2. **EOF final line.** A script ending without a newline yields one
   extra trailing prompt (`zhex> ` or `....> `) in the prototype: the
   partial final line is a successful `read-line`, so the driver learns
   EOF only on the next read. Accepted by the frozen decisions.
3. **Invalid UTF-8 input.** `read-line` validates UTF-8 and fails the
   read (driving the EOF policy), while piped production passes raw
   bytes onward to the parser's source error. The matrix uses valid
   UTF-8 scripts only.

## Recorded gaps (requirements versus conveniences)

Requirement for any production replacement: a stderr/diagnostics writer
reachable from Zhex, and terminal parity (editor, Ctrl-C/Ctrl-D, paste,
restoration) as its own gate — the existing PTY suite stays the
boundary. Conveniences deferred: editor indent probing
(`DelimiterIndent` composition) is deliberately outside this seam; a
driver-side transcript/history is unnecessary while export is
engine-side; no guard bounds nested `repl.enter` recursion (each nested
entry gets a fresh runtime, so the engine's depth limit does not apply —
a self-referential script can exhaust the Go stack).

## Invocation

```
go build -o dogfood ./tools/dogfood && ./dogfood < script
```

Exit status propagates from `bye()`/`console.exit`/`console.fail`.
