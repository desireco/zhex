# Initial Zhex examples

These source files are runnable Zhex programs validated by the interpreter's test suite. They exercise readable composition, named values, conditions, loops, console input/output, records, choices, collections, and explicit error handling. The [initial language contract](initial-language.md) defines their syntax and semantics.

## Language basics

| File | Input | Result after the prompt | Purpose |
| --- | --- | --- | --- |
| [hello.zhex](../examples/hello.zhex) | `Ada` | `Hello, Ada!` | Read a whole line, build a string, print it. |
| [composition.zhex](../examples/composition.zhex) | None | `32` | Typed words, inferred locals, flow composition. |
| [with-blocks.zhex](../examples/with-blocks.zhex) | None | `he`, `3` | With blocks: a fixed seed under `with text` and a fused `with list` form, every step receiving the seed. |
| [calls.zhex](../examples/calls.zhex) | None | `Hello, Ada`, `Hello, Grace`, `3`, `3`, `7`, `2`, `1`, `3` | Simple and parenthesized calls, defaults, zero-argument words, and cases that still need parentheses. |
| [word-defaults.zhex](../examples/word-defaults.zhex) | None | `Hello Grace`, `default ran`, `Hello Ada`, `1`, `1` | Infer a parameter type from a default, evaluate it only when omitted, and give each call a fresh mutable default. |
| [partials.zhex](../examples/partials.zhex) | None | `12`, `12`, `12`, `1`, `1`, `12`, `aDa`, `7`, `1`, `8`, `text-to-int`, `3` | Word and built-in partials, typed function values, captures, fresh deferred defaults, a flow step, and invocation-time error handling. |
| [records.zhex](../examples/records.zhex) | None | `Ada, age 36` | A nominal record declaration, a positional constructor, typed binding, field reads, and a word that accepts the record. |
| [choices.zhex](../examples/choices.zhex) | None | `running`, `Could not start: network unavailable`, `7`, `0` | Concrete and one-parameter alternatives, payload binding, contextual nullary types, and exhaustive matching with a default. |
| [collections.zhex](../examples/collections.zhex) | None | `3` and `Grace`, then `3` — the length after a flow chains two `list.append` steps — then list indices and array coordinates, each append reports `collection-iterating`, `2`, `Lin` from `list.select`, `1`, `5` from `array.select`, then `6`, `7`, `12` after resize | Shared list mutation and indexing, a flow chaining `list.append` steps (mutation words return their receiver), dual-binding traversal, predicate selection of elements and rows, row-major array access, pre-mutation guards, overlap-preserving resize, and fill values. |
| [maps.zhex](../examples/maps.zhex) | None | `1`, `3`, `Ada: 11`, `Morgan has no score`, `Ada: 11`, `Grace: 9`, `Nancy: 23`, `4`, `4` | Inferred and empty typed maps, optional lookups, insertion order, update/removal, and map operations composed in flows. |
| [temporal.zhex](../examples/temporal.zhex) | None | Noon and 1 p.m. after the DST transition | Calendar-day and elapsed-duration arithmetic in a named zone. |
| [json-config.zhex](../examples/json-config.zhex) | None | `3`, `true`, `{"retries":3,"verbose":true}` | Decode a typed record with a default for a missing field, then encode the complete value. |
| [json-tree.zhex](../examples/json-tree.zhex) | None | `nodes: 12`, each dynamic key with its rendered value, then the compact round-trip text | Parse a heterogeneous document into the `Json` tree type, count nodes recursively, label every constructor, and encode the tree back. |
| [xml-config.zhex](../examples/xml-config.zhex) | None | `ingest`, `3`, `true`, `2`, `db:5432`, `owner unset` | Decode an embedded XML document into records: attributes map onto fields, repeated elements collect into a list, and the absent owner stays `Option.None`. |
| [text-stats.zhex](../examples/text-stats.zhex) | None (argument: `examples/text-stats-sample.txt`) | `3 lines  11 words  64 chars` | Program arguments, whole-file counts, and streamed uppercased lines. |
| [line-filter.zhex](../examples/line-filter.zhex) | None | `shipped: inv-1042, inv-1044` | Split embedded records, select lines by literal prefix and suffix, and join the selected fields into a report. |
| [score-report.zhex](../examples/score-report.zhex) | None | The joined names, then `passing: 3 of 4`, `total: 100`, `first above 40: 41`, and the any/all verdict | Trim and score embedded records through `list.map`, filter a subset, fold a total, and answer find, any, and all questions. |
| [sorting.zhex](../examples/sorting.zhex) | None | The ranked entries with equal scores in source order, then `source unchanged: Ada: 41`, then the sorted fruits | Rank records with `list.sort-by` and a named key word, keep stable ties, and sort plain text with `list.sort`; neither word mutates its source. |
| [file-write.zhex](../examples/file-write.zhex) | None | `Save failed: file-write`, then the in-memory report | Attempt `file.write` on a path the host refuses, and inspect the caught failure with `error.code`. |
| [paths.zhex](../examples/paths.zhex) | None | The joined invoice path, then its directory, base, and extension, then the replaced and cleaned variants | Compose paths with `file.join` (absolute child replaces the base, `..` cleans lexically) and decompose them with `file.base`, `file.extension`, and `file.directory-of`. |
| [file-work.zhex](../examples/file-work.zhex) | None (argument: `.`) | The folder's two entry names, the invoice byte count, then the removal verdict | The destructive round trip: `make-folder`, two `file.write` calls, `list`, `size`, `free-space`, `delete`, and `delete-folder` with `recursive`. |
| [env-config.zhex](../examples/env-config.zhex) | None (argument: `.`) | The loaded title, the set-but-empty mode, the missing limit, then `report configured` | Write a report.env fixture, load it with `env.load` (first assignment wins), and read each setting back through `env.get` Option matches. |
| [recovery.zhex](../examples/recovery.zhex) | None | A fallback message, then `Retries: 3` | Catch invalid numeric text and continue with a default value. |
| [exit-status.zhex](../examples/exit-status.zhex) | `0` | `Exiting with status 0` after the prompt | Validate a requested status, recover malformed input in `catch`, and terminate with `console.exit`. |
| [modules/main.zhex](../examples/modules/main.zhex) | `48`, `18` | `30`, `6`, `144` | A two-file program: `use math` imports the sibling `math.zhex`, calls its words qualified, and keeps one helper underscore-private. |
| [modules/md-report/main.zhex](../examples/modules/md-report/main.zhex) | None | The escaped heading and paragraph, the aligned pipe table, both lists, the link, and the fenced code block | A two-file program: `use md` imports the sibling `md.zhex` and renders records and lists into a Markdown report accumulated in a buffer. |
| [modules/md-report/md.test.zhex](../examples/modules/md-report/md.test.zhex) | None | `md parse, section editing, round-trip, and html export tests passed` | A three-file program: the md module's `md.parse` slices, section and block editing, the round-trip and canonicalization laws, and the HTML export, checked through the sibling `test` module's `check`. |
| [modules/html-page/main.zhex](../examples/modules/html-page/main.zhex) | None | A complete HTML status page | A two-file program over the `html` composition module: `escape-text` and `attr` escape text by default, markup nests through `element` with one deliberate `html.raw`, and `link` rejects a `javascript:` target. |
| [modules/math.test.zhex](../examples/modules/math.test.zhex) | None | `math module tests passed` | A three-file program: `use test` imports the sibling one-word assertion module, `check(condition, label)` fails the run with its label when a condition is false, and the `zhex test` runner discovers every such `*.test.zhex` file. |
| [modules/epub-book/main.zhex](../examples/modules/epub-book/main.zhex) | None | `chapters: 2`, `book bytes: 2644`, and `mimetype first: 97` | A three-file program: `use epub` assembles a two-chapter EPUB 3 archive over the pure-Zhex `zip` writer and prints the book's facts read from the raw archive bytes, all in memory. |
| [modules/epub-book/epub.test.zhex](../examples/modules/epub-book/epub.test.zhex) | None | `epub module tests passed` | A four-file program: the epub module's UTF-8 encoder vectors, container/OPF/nav XML parts, mimetype-first ordering, and whole-book bytes, checked through the sibling `test` module's `check`. |
| [modules/epub-book/zip.test.zhex](../examples/modules/epub-book/zip.test.zhex) | None | `zip module tests passed` | A three-file program: the zip module's known CRC32 vectors, a byte-pinned single-entry archive, and the empty edges. |
| [modules/pdf-doc/main.zhex](../examples/modules/pdf-doc/main.zhex) | None (argument: `.`) | `chapters: 3`, `pages: 4`, the ASCII character count, `header: %PDF-1.4`, `trailer: %%EOF ends the file`, `font: Helvetica base-14`, and `wrote pdf-book.pdf` | A two-file program: `use pdf` hand-emits a three-chapter PDF book with wrapped paragraphs, 24pt chapter titles, and page breaks, writes it into the argument folder, and removes it. |
| [modules/pdf-doc/pdf.test.zhex](../examples/modules/pdf-doc/pdf.test.zhex) | None | `pdf module tests passed` | A three-file program: the pdf module's objects, xref offsets, WinAnsi-escaped text, and whole-document bytes, pinned in memory. |
| [modules/text-diff/main.zhex](../examples/modules/text-diff/main.zhex) | None | The review-style excerpt of `-`/`+`/kept lines and `patch law holds: the replayed diff reproduces the revision` | A two-file program: `use diff` computes the Myers line diff between two versions of a note, prints it review-style, and applies it back onto the old text. |
| [modules/text-diff/diff.test.zhex](../examples/modules/text-diff/diff.test.zhex) | None | Nothing — a failed `test.check` label is the failure | A three-file program: the diff module's kept, deleted, and inserted lines, the patch law over generated texts, and the checked apply channel's misalignment failures. |
| [modules/writer/main.zhex](../examples/modules/writer/main.zhex) | None | The draft's outline, the sentence and word answers, the stats, the outline after the edits, and the draft back as markdown and an HTML fragment | The writer's showcase: a draft loads through the md module, passes run over the sentence and word layers with the ordinary list words, edits land by section and block, and the result publishes as markdown and HTML. |
| [modules/writer/writer.test.zhex](../examples/modules/writer/writer.test.zhex) | None | `writer showcase tests passed` | A three-file program: the showcase's outline, passes, edits, and published forms, checked through the sibling `test` module's `check`. |

## Algorithms

These programs use the same checked values, loops, words, and flows to express familiar algorithms. Each source is shown in full and runs in the test suite.

| File | Input | Result after the prompt | Purpose |
| --- | --- | --- | --- |
| [fizzbuzz.zhex](../examples/fizzbuzz.zhex) | `15` | The sequence below | Remainders, ordered conditions, bounded iteration, formatting. |
| [fibonacci.zhex](../examples/fibonacci.zhex) | `10` | `55` | Explicit local mutation and iterative computation. |
| [factorial.zhex](../examples/factorial.zhex) | `5` | `120` | Accumulation, empty loops, integer limits. |
| [palindrome.zhex](../examples/palindrome.zhex) | `racecar` | `Palindrome` | String input, comparison, and a text-library operation. |
| [gcd.zhex](../examples/gcd.zhex) | `48`, `18` | `6` | Euclid's algorithm by recursion and a flow seeded with a computed call. |
| [times-table.zhex](../examples/times-table.zhex) | `7` | `7 x 1 = 7` through `7 x 10 = 70` | A bounded loop building each line from integers and strings. |
| [life.zhex](../examples/life.zhex) | None | Five generations of a 7×7 glider | Apply Conway's neighbor rules to a rectangular array; cells beyond the board are dead. |
| [edit-distance.zhex](../examples/edit-distance.zhex) | None | `kitten → sitting: 3`, `empty → abc: 3`, `café → cafe: 1` | Fill a dynamic-programming table for insertion, deletion, and substitution distance over Unicode code points. |
| [collatz.zhex](../examples/collatz.zhex) | None | `1: 0` through `9: 19` | A `while` loop that terminates by its condition, with branch updates to a mutable counter. |

Each result line ends with a newline. The interactive programs print a prompt before reading. For FizzBuzz with input `15`, the result lines are:

```text
1
2
Fizz
4
Buzz
Fizz
7
8
Fizz
Buzz
11
Fizz
13
14
FizzBuzz
```

The collections example first chains `list.append` steps in a flow (mutation words return their receiver, so the list threads through) and prints the grown length, then also prints the zero-based list index with each name, reports the guarded append, selects the short names with `list.select` and the rows whose first cell is big with `array.select` (one row per call as a `List<Int>`, rectangle preserved), then visits the 2×3 array in row-major coordinate order before showing preserved and new values after resize:

```text
3
Grace
3
0: Ada
collection-iterating
1: Grace
collection-iterating
2: Lin
collection-iterating
0,0: 1
0,1: 2
0,2: 3
1,0: 4
1,1: 5
1,2: 6
6
7
12
```

## Input and output

| Operation | Contract |
| --- | --- |
| `read-line(prompt)` | Write and flush the prompt, then return `Result<String, InputError>` from one line of standard input. |
| `text.to-int(text)` | Trim surrounding whitespace and parse an optional sign followed by decimal digits into a checked 128-bit `Int`; return `Result<Int, ParseError>`. |
| `print(value)` | Write an `Int`, `String`, or `Bool` to standard output followed by a newline; return `Unit`. |

`read-line` removes one trailing LF and an immediately preceding CR, preserving the remaining spaces and text. An empty line is a valid empty string. A nonempty final line without a newline is accepted. EOF before any line content, invalid UTF-8, and I/O failures produce input errors. `text.to-int` additionally rejects empty/whitespace-only input, malformed integers, and integers outside the supported range.

The interactive examples use postfix `try` to propagate failed input to the program boundary; the runner writes a source-located diagnostic and exits nonzero. Earlier runtime output, including a prompt, can remain. `recovery.zhex` demonstrates a local `catch` fallback: invalid numeric text prints a short explanation and supplies `3`, so the program continues. For declared fallible words, propagation across multiple calls, and structured error reporting, see [results and error handling](../site/src/content/docs/results.md).

All names and types are checked before the program runs, so an unknown word or incompatible flow produces no prompt and consumes no input. Output failures should likewise produce controlled runtime errors. Input comes from standard input in both interactive terminals and redirected streams; file execution and the REPL must share that input stream without losing buffered data.

Run the examples:

```sh
zhex run examples/hello.zhex
zhex run examples/composition.zhex
zhex run examples/records.zhex
zhex run examples/choices.zhex
zhex run examples/collections.zhex
zhex run examples/maps.zhex
zhex run examples/json-config.zhex
zhex run examples/json-tree.zhex
zhex run examples/xml-config.zhex
zhex run examples/fizzbuzz.zhex
zhex run examples/fibonacci.zhex
zhex run examples/factorial.zhex
zhex run examples/palindrome.zhex
zhex run examples/gcd.zhex
zhex run examples/times-table.zhex
zhex run examples/text-stats.zhex examples/text-stats-sample.txt
zhex run examples/recovery.zhex
printf '0\n' | zhex run examples/exit-status.zhex
zhex run examples/modules/main.zhex
zhex run examples/modules/html-page/main.zhex
zhex run examples/modules/math.test.zhex
```

## Syntax exercised by the examples

- Word parameter annotations may be omitted when body and same-program call constraints determine one concrete type; unresolved parameters need annotations, conflicting calls are type errors, and `Result` is never a parameter type. A return annotation is optional when the body determines one concrete success type. An escaping `try` infers a user word's effective `Result<T, E>` signature; explicit Result signatures remain available for `ok`/`err` construction. Local types and flow intermediate types are inferred.
- `let` binds an immutable local. `var` explicitly permits reassignment within lexical scope; assignment preserves its type. Naming a value does not dictate its eventual machine representation.
- Blocks use indentation. A word's final expression supplies its result, except in a word declared `-> Unit`, whose last line is an ordinary line like its other lines. A value that nothing reads is dropped: an `if` or `match` on an ordinary line may let its branches end with any type, while one whose value is used — as a word's result, or as the value of a binding or assignment — must have branches that end with the same type.
- Calls may use parentheses or the simple comma-separated form at a complete expression boundary. Qualify type- and module-owned operations such as `text.reverse`, `math.gcd`, and `error.code`; hyphenated words such as `is-palindrome` are identifiers. Use spaces around subtraction between names: `a-b` is a name, while `a - b` subtracts.
- `+` supports integer addition or string concatenation with matching operand types; the one exception is the string-context Int — a `String` concatenated with an `Int` renders the Int as `int.to-string` does, and `print` accepts an `Int` directly (#83). `%` is integer remainder. `==` compares same-typed values by value. `or` combines booleans with short-circuit evaluation.
- `range(start, end)` is an ascending integer iteration with an exclusive end. When `start >= end`, it performs no iterations. It should not require materializing a list of all integers.
- `int.to-string` returns decimal text. `text.reverse` reverses a valid string by Unicode code points. These standard operations are predeclared.

`print` uses statically selected built-in overloads rather than requiring a general formatting protocol. Boolean output is `true` or `false`. These examples use concrete value types and monomorphic words rather than general traits or polymorphism.

## Boundary and error examples

| Program | Case | Expected behavior |
| --- | --- | --- |
| Hello | Empty line | Print `Hello, !`. |
| FizzBuzz | `1` | Print `1`. |
| FizzBuzz | `0`, negative, or above `10000` | Print the supported-range message and skip the calculation. |
| Fibonacci | `0`, `1`, `2` | Print `0`, `1`, `1`, respectively. |
| Fibonacci | `92` | Print `7540113804746346429` without computing the overflowing next Fibonacci number. |
| Fibonacci | Negative or above `92` | Print the supported-range message and skip the calculation. |
| Factorial | `0`, `1` | Print `1`. |
| Factorial | `20` | Print `2432902008176640000`. |
| Factorial | Negative or above `20` | Print the supported-range message and skip the calculation. |
| Palindrome | `racecar`, `abba`, `été`, or an empty line | Print `Palindrome`. |
| Palindrome | `hello` or `Racecar` | Print `Not a palindrome`. |
| Numeric input in existing examples | `abc`, `1.5`, empty line, or out-of-range integer | The example uses top-level `try`, so the runner reports an input error and exits nonzero. |
| Any input | EOF before any content | Report an input error and exit nonzero; never silently substitute zero or an empty string. |

Supported-range messages are successful program branches and exit normally; malformed input is a Result failure. In programs that use top-level `try`, it becomes a runtime diagnostic. A `catch` handler can instead recover a same-typed value, inspect `error.code`, and optionally call a registered reporter with `error.report`. Fibonacci uses `F(0) = 0` and `F(1) = 1`. The example runner bounds avoid signed 64-bit overflow; the arithmetic operations must still detect overflow independently. The mathematical helper words assume the documented input domain and are guarded by their callers here.

Palindrome comparison is exact, including case, punctuation, spaces, and Unicode normalization. Code-point reversal does not group combining sequences into visual characters. Case folding, normalization, and grapheme-aware operations can be added as explicit text-library features later.

## Later algorithm examples

GCD has landed — [gcd.zhex](../examples/gcd.zhex) runs Euclid's algorithm by recursion — and [text-stats.zhex](../examples/text-stats.zhex) is the first real application, exercising the file and text vocabulary on a sample document. Primality testing would exercise early exits; exponentiation by squaring would exercise logarithmic iteration. Binary search could show array indexing in an algorithm, while word-frequency counting could apply the existing map operations to text processing. Add these when they make useful runnable examples.
