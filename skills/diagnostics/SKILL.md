---
name: zhex-diagnostics
description: Repair guide for Zhex diagnostic codes — use when a zhex report or diagnostic names a code; one anchored section per code with its meaning, fix, and learn URL, generated from the catalog inside the tool.
---

<!-- GENERATED from internal/lang/catalog.md — edit the catalog, then run `go generate ./internal/lang`; never edit this file by hand. -->

# Zhex diagnostics repair guide

## Workflow

- Zhex checks the whole program before anything runs. Run `zhex run <file> --report=json` and read each entry `{code, severity, file, line, column, message, hint}` (plus additive `span_start_column`/`span_end_column` on checker diagnostics and `url` on catalog codes); a clean run means names, types, and word connections resolved.
- Apply the message and its `hint:` line — the hint names the action — and use the code's section below for the meaning and the fix. Rerun until the report is clean.
- Offline, `zhex errors <code>` prints the code's full catalog entry with its learn URL; `zhex errors` lists every code, `zhex errors --json` emits the catalog as JSON, and an unknown code suggests the closest.
- To watch what a program did instead of its failures, add `--trace`: JSONL step events stream to stderr before any diagnostic, and the flags combine (events, then the report).

## addition-type

`+` adds two `Int`s, concatenates two `String`s, or concatenates a `String` and an `Int` — the string-context Int exception.

Fix: Other equal operand types, such as two `Bool`s, cannot use `+`; convert explicitly. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#addition-type

## argument-count

A word call passed the wrong number of arguments.

Fix: The message states how many the word takes.

Learn: https://desireco.github.io/zhex/errors.html#argument-count

## argument-missing

`console.arg` received an index outside the program's command-line arguments.

Fix: The message names the index and how many arguments were given; the failure lives inside the `Result` and surfaces through `try`. Check the index against `console.args-count()`.

Learn: https://desireco.github.io/zhex/errors.html#argument-missing

## argument-type

An argument's type does not match the parameter.

Fix: There are no implicit conversions; convert at the call site. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#argument-type

## arithmetic-type

`- * / %` need `Int` operands.

Fix: When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#arithmetic-type

## array-range

An array coordinate is outside its dimension.

Learn: https://desireco.github.io/zhex/errors.html#array-range

## array-rank

An array shape or index has the wrong rank.

Learn: https://desireco.github.io/zhex/errors.html#array-rank

## array-shape

An array extent is negative.

Learn: https://desireco.github.io/zhex/errors.html#array-shape

## assignment-type

A binding annotation must match its value's type.

Fix: The new value's type differs from the variable's type. There are no implicit conversions; convert explicitly (for example `int.to-string`). When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#assignment-type

## bool-operand

`and`/`or` need `Bool` on both sides.

Fix: Comparisons produce `Bool`; integers do not. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#bool-operand

## branch-type

The branches of an `if` or the cases of a `match` end in different types where the value is read.

Fix: Give both arms the same type, or put the block on an ordinary line to drop what the arms produce.

Learn: https://desireco.github.io/zhex/errors.html#branch-type

## buffer-range

`buffer.line` or `buffer.set-line` received an index outside the buffer's lines.

Fix: The failure lives inside the `Result` and surfaces through `try`; check the index against `buffer.line-count` first.

Learn: https://desireco.github.io/zhex/errors.html#buffer-range

## collection-iterating

An active iteration prevents mutation of the traversed collection.

Fix: Mutate a different collection or wait until traversal finishes.

Learn: https://desireco.github.io/zhex/errors.html#collection-iterating

## collection-limit

A list or array would exceed the runtime limit of 1,000,000 elements.

Fix: Reduce its size.

Learn: https://desireco.github.io/zhex/errors.html#collection-limit

## compare-type

Equality comparison needs matching `Int`, `Bool`, or `String` values.

Fix: If one side is a `Result`, add postfix `try` to unwrap it. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#compare-type

## condition-type

An `if` condition must be `Bool`.

Fix: Comparisons return `Bool`; integers and strings do not. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#condition-type

## console-exit-status

`console.exit` requires a status from 0 through 255.

Fix: Choose a portable exit status in that range.

Learn: https://desireco.github.io/zhex/errors.html#console-exit-status

## console-input

Reading a line from the input stream failed, usually because the input ended.

Fix: The `reason` detail is `end-of-file` when the input ended (Ctrl-D or piped EOF; an empty line is a success, not this error) and `read-failed` for a read failure; read it with `error.detail` to tell them apart. The failure propagates through `try` and exits the program; check that input is available.

Learn: https://desireco.github.io/zhex/errors.html#console-input

## console-output

Writing to the output stream failed.

Fix: Check that the output is writable (closed pipes fail this way).

Learn: https://desireco.github.io/zhex/errors.html#console-output

## console-utf8

The line read from the input stream is not valid UTF-8.

Fix: Zhex reads UTF-8 text; supply the input as UTF-8.

Learn: https://desireco.github.io/zhex/errors.html#console-utf8

## date-parse

A date string is invalid.

Fix: Use a Gregorian date in `YYYY-MM-DD` form.

Learn: https://desireco.github.io/zhex/errors.html#date-parse

## depth-limit

Word recursion passed the runtime depth limit of 256.

Fix: Rewrite the recursion as a loop with `var` and `for`; loops have no depth limit.

Learn: https://desireco.github.io/zhex/errors.html#depth-limit

## division-zero

Division or remainder by zero.

Fix: Guard the divisor with a condition first.

Learn: https://desireco.github.io/zhex/errors.html#division-zero

## doctest-check

A `## >` example's expression failed to check or evaluate by itself.

Fix: Fix the expression so it checks and evaluates — a naked fallible call needs `try`, and a Result cannot be echoed bare.

Learn: https://desireco.github.io/zhex/errors.html#doctest-check

## doctest-mismatch

A `## >` example in a word's doc produced a different value than the expected output lines after it (they run until a blank doc line or the next `## >` line).

Fix: Update the expected output, or fix the word. Checked by `zhex doctests <file>`; `zhex run` never executes docs.

Learn: https://desireco.github.io/zhex/errors.html#doctest-mismatch

## duplicate-name

This name is already defined in the same scope.

Fix: Choose a fresh name; shadowing is not allowed.

Learn: https://desireco.github.io/zhex/errors.html#duplicate-name

## duplicate-word

A word with this name already exists, or it collides with a host word.

Fix: Rename one of them.

Learn: https://desireco.github.io/zhex/errors.html#duplicate-word

## duration-parse

A duration string is invalid or outside the supported nanosecond range.

Learn: https://desireco.github.io/zhex/errors.html#duration-parse

## empty-block

A block header is followed by nothing.

Fix: Every block needs at least one statement.

Learn: https://desireco.github.io/zhex/errors.html#empty-block

## env-parse

A line of the file given to `env.load` is not a KEY=VALUE assignment: the `=` is missing, the key is empty, or the key contains spaces.

Fix: The message names the line number and the offending text. Lines before the malformed one are already applied and stay applied — env.load never overrides, so rerunning after fixing the file keeps the earlier values.

Learn: https://desireco.github.io/zhex/errors.html#env-parse

## env-read

`env.load` could not read the file: it is missing, or the process may not read it.

Fix: The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#env-read

## error-no-cause

`error.cause` was called for an Error without a cause.

Fix: Check `error.has-cause` first or handle the Result.

Learn: https://desireco.github.io/zhex/errors.html#error-no-cause

## expected-expression

A token cannot start an expression where one is required.

Learn: https://desireco.github.io/zhex/errors.html#expected-expression

## expected-indent

A construct needs an indented block on the following lines.

Fix: Indent the body four spaces. Open `()`, `[]`, and `{}` delimiters can continue expressions across lines, but do not replace a required block body.

Learn: https://desireco.github.io/zhex/errors.html#expected-indent

## expected-name

A keyword appears where a name is required.

Fix: `word`, `let`, `var`, `if`, and the other keywords are reserved; choose a name that is not one of them.

Learn: https://desireco.github.io/zhex/errors.html#expected-name

## expected-token

The parser expected a specific token and found something else.

Fix: The message names both; fix the line to match the expected grammar. When a `[` follows `List`, `Array`, `Map`, `Option`, or `Result` in a type annotation or result signature, the hint names the angle-bracket spelling: write `Result<Int, Error>`, not `Result[Int, Error]`.

Learn: https://desireco.github.io/zhex/errors.html#expected-token

## file-bytes-range

`file.write-bytes` received a byte value outside 0 through 255; the message names the first offending value.

Fix: The failure lives inside the `Result`, is checked before the file is opened, and never creates or truncates a file.

Learn: https://desireco.github.io/zhex/errors.html#file-bytes-range

## file-delete

`file.delete` could not delete the path: it is missing, it is a nonempty folder (the safety default removes a file or an empty folder only; `file.delete-folder(recursive: true)` removes a tree), or the process may not remove it.

Fix: The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-delete

## file-delete-folder

`file.delete-folder` could not remove the folder: it holds entries while `recursive` is false (the default removes an empty folder only; pass `recursive: true` for the whole tree), or the process may not remove something inside it.

Fix: Recursive removal is best-effort: part of the tree may already be gone when the first failure is reported, and earlier effects stand. The message names the folder and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-delete-folder

## file-exists

`file.exists` could not answer for the path: the message names the path and the operating system's reason, and the failure carries the not-found/permission-denied/other taxonomy in its details.

Fix: A missing path is not this failure — it is the answer `false`; apply `try` or `catch` to inspect the checked form's failures.

Learn: https://desireco.github.io/zhex/errors.html#file-exists

## file-free-space

`file.free-space` could not report the bytes available on the volume containing the path: the path does not exist, or the platform cannot answer for it.

Fix: The message names the path and the reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-free-space

## file-list

`file.list` could not list the folder: the folder is missing, or the process may not read it, or the path is a file.

Fix: The message names the folder and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-list

## file-make-folder

`file.make-folder` could not create the folder: a parent is missing (pass `parents: true` to create ancestors), or the target already exists (the default one-level form refuses it; `parents: true` tolerates an existing folder), or the process may not write the parent.

Fix: The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-make-folder

## file-move

`file.move` could not rename the source to the target: the source is missing, the target exists (pass `overwrite: true` to replace an existing file), the target is a folder (never replaced in either mode), the two paths are on different devices (no copy fallback), or the process may not rename.

Fix: The check-then-act window between the target check and the rename is inherent: a target that appears in between is left to the rename itself. The message names both paths and the reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-move

## file-open

`file.open`, `file.read`, or a `file.stream` loop could not open the path.

Fix: The message names the path and the operating system's reason; check the path and its permissions.

Learn: https://desireco.github.io/zhex/errors.html#file-open

## file-read

A read from an open file failed: the stream was drained ("end of file") or the file was closed.

Fix: `file.read-line` at end of file fails inside the `Result`; to read a whole file, prefer `file.read(path)` or `for line in file.stream(path)`.

Learn: https://desireco.github.io/zhex/errors.html#file-read

## file-size

`file.size` could not get the size of the path: it is missing, or the process may not stat it.

Fix: The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

Learn: https://desireco.github.io/zhex/errors.html#file-size

## file-utf8

A file's content is not valid UTF-8.

Fix: Zhex reads UTF-8 text only; re-save the file as UTF-8.

Learn: https://desireco.github.io/zhex/errors.html#file-utf8

## file-write

`file.write` could not open, write, or close the path; the message names the path, the failing phase, and the operating system's reason.

Fix: The failure lives inside the `Result` and surfaces through `try`; a failed write may leave a partial or truncated file, and a missing parent directory is never created.

Learn: https://desireco.github.io/zhex/errors.html#file-write

## flow-step

A `flow` block needs at least one step word.

Fix: Write one word per line; the first receives the seed value.

Learn: https://desireco.github.io/zhex/errors.html#flow-step

## immutable-name

A name defined with `let` cannot be reassigned.

Fix: Declare it with `var` to allow updates.

Learn: https://desireco.github.io/zhex/errors.html#immutable-name

## indentation

A line's indent is not a multiple of four spaces, or it opens more than one level at once.

Fix: Each block indents exactly four spaces from its parent.

Learn: https://desireco.github.io/zhex/errors.html#indentation

## instant-parse

An instant string is invalid.

Fix: Include a date, time, and explicit RFC 3339 offset.

Learn: https://desireco.github.io/zhex/errors.html#instant-parse

## int-shift-count

A shift word's count is negative.

Fix: Shift counts are zero or more; compute the count from nonnegative values or guard it with a condition first.

Learn: https://desireco.github.io/zhex/errors.html#int-shift-count

## int-shift-width

A shift count of 64 or more in a build whose Int is 64 bits wide: compiled WebAssembly and the browser engine.

Fix: Keep the count below 64 there, or use the interpreter or a native build, whose Int carries the full 128-bit width.

Learn: https://desireco.github.io/zhex/errors.html#int-shift-width

## integer-literal-range

An integer literal is outside the signed 128-bit range.

Fix: Keep literals within ±170141183460469231731687303715884105727; compute larger values at runtime.

Learn: https://desireco.github.io/zhex/errors.html#integer-literal-range

## integer-overflow

An arithmetic result does not fit in a signed 128-bit integer.

Fix: Overflow is an error in Zhex, not a wraparound; keep intermediate values in range.

Learn: https://desireco.github.io/zhex/errors.html#integer-overflow

## json-depth

JSON nesting exceeds the supported depth of 256.

Fix: Reduce the nesting.

Learn: https://desireco.github.io/zhex/errors.html#json-depth

## json-field

A JSON object has an unknown, duplicate, or missing required field.

Fix: Inspect the path and reason.

Learn: https://desireco.github.io/zhex/errors.html#json-field

## json-range

A JSON integer or collection exceeds the supported range.

Fix: Correct the value or reduce the list size.

Learn: https://desireco.github.io/zhex/errors.html#json-range

## json-syntax

JSON text is invalid or contains more than one value.

Fix: Correct the input syntax.

Learn: https://desireco.github.io/zhex/errors.html#json-syntax

## json-type

A JSON value does not match the expected Zhex type.

Fix: Inspect the path and correct the value.

Learn: https://desireco.github.io/zhex/errors.html#json-type

## list-range

The list index is negative or outside its current length, or the list is empty where `list.first`, `list.last`, or `list.pop` needs an element.

Fix: Use a valid zero-based index, or handle the empty case with `catch` or a `list.length` check.

Learn: https://desireco.github.io/zhex/errors.html#list-range

## main-parameters

A file whose top level has no statements names `word main` as its entry point, and `zhex run` and `zhex compile` call it with no arguments.

Fix: Remove the parameter list from `word main`, or rename the word. Beside top-level statements, in an imported file, and in the REPL, `main` is an ordinary word name.

Learn: https://desireco.github.io/zhex/errors.html#main-parameters

## module-body

An imported file contains top-level statements.

Fix: Imported files declare words only; move the statements into the file you run — a zero-argument `word main` there is invoked as the entry point.

Learn: https://desireco.github.io/zhex/errors.html#module-body

## nesting-limit

Expressions or blocks nest past the parser's limit.

Fix: Split the expression into named parts with `let`.

Learn: https://desireco.github.io/zhex/errors.html#nesting-limit

## not-available-in-build

The build that ran the program does not provide this construct.

Fix: `zhex compile` fences constructs its emitter does not support yet; the browser engine build has no filesystem or console arguments (`file.*` filesystem words, `file.stream`, `console.arg`, `env.load`). Run the program with `zhex run`, or rewrite it with constructs the build supports.

Learn: https://desireco.github.io/zhex/errors.html#not-available-in-build

## operand-types

A binary operator got two different operand types.

Fix: If one side is a `Result`, add postfix `try` to unwrap it. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#operand-types

## ordering-type

Ordering comparison (`< <= > >=`) needs `Int` operands.

Fix: When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#ordering-type

## print-type

`print` accepts one `Int`, `String`, or `Bool` value.

Fix: If it received a `Result`, add postfix `try` to unwrap it. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#print-type

## private-use

A word or record type beginning with an underscore is private to its module.

Fix: Public declarations cannot expose a private record type. Make the declaration private or use a public type.

Learn: https://desireco.github.io/zhex/errors.html#private-use

## range-context

`range` and `file.stream` exist only inside a `for` loop header — `for n in range(start, end)` and `for line in file.stream(path)`.

Fix: They cannot be called elsewhere.

Learn: https://desireco.github.io/zhex/errors.html#range-context

## range-type

The bounds of `range(start, end)` must both be `Int`; the path of `file.stream(path)` must be `String`.

Learn: https://desireco.github.io/zhex/errors.html#range-type

## replace-empty

`text.replace` was called with an empty old text.

Fix: Every position matches an empty string, so there is no sane result; this is a bug signal, not a recoverable case. Pass a nonempty old text.

Learn: https://desireco.github.io/zhex/errors.html#replace-empty

## source-utf8

The file is not valid UTF-8.

Fix: Re-save it as UTF-8; Zhex source has no other encoding.

Learn: https://desireco.github.io/zhex/errors.html#source-utf8

## stack-range

The stack is empty where `stack.pop`, `stack.peek`, `stack.dup`, or `stack.drop` needs the top element, or has fewer than two elements where `stack.swap` needs the top pair.

Fix: Check `stack.length` first, or handle the empty case with `catch`.

Learn: https://desireco.github.io/zhex/errors.html#stack-range

## string-escape

A string contains an escape Zhex does not support, or invalid UTF-8.

Fix: Strings are double-quoted and use Go-style escapes like `\n` and `\"`.

Learn: https://desireco.github.io/zhex/errors.html#string-escape

## tab-character

A tab appeared outside a string.

Fix: Indentation is four spaces per level; replace tabs with spaces.

Learn: https://desireco.github.io/zhex/errors.html#tab-character

## temporal-invalid

Numeric date or time components are outside their valid range.

Learn: https://desireco.github.io/zhex/errors.html#temporal-invalid

## temporal-range

Temporal arithmetic produced a value outside the supported range.

Learn: https://desireco.github.io/zhex/errors.html#temporal-range

## test-failed

A `.test.zhex` file run by `zhex test` exited nonzero without a diagnostic of its own — a failed `test.check` label, a deliberate exit, or a crash.

Fix: Run `zhex run <file>` to see the failing check in place.

Learn: https://desireco.github.io/zhex/errors.html#test-failed

## text-code-point

`text.from-code-points` received a code point that is not valid Unicode: negative, above U+10FFFF, or a surrogate half (U+D800..U+DFFF).

Fix: Strings are UTF-8 text, so those values have no encoding; this is a bug signal, not a recoverable case. Check the numbers before building the list.

Learn: https://desireco.github.io/zhex/errors.html#text-code-point

## text-pad-fill

`text.pad-left` or `text.pad-right` received a fill that is not exactly one code point.

Fix: Padding reaches an exact width only by repeating a single character, and an empty or multi-code-point fill cannot land on an arbitrary width. Pass a one-code-point fill, or omit it to pad with spaces.

Learn: https://desireco.github.io/zhex/errors.html#text-pad-fill

## text-pad-width

`text.pad-left` or `text.pad-right` received a negative width.

Fix: The width is the target length, so a negative value has no meaning; this is a bug signal, not a recoverable case. Pass a width of zero or more — padding never truncates, so a width at or below the text's length returns the text unchanged.

Learn: https://desireco.github.io/zhex/errors.html#text-pad-width

## text-repeat-count

`text.repeat` received a negative count.

Fix: A negative count has no meaning — zero already produces the empty string; this is a bug signal, not a recoverable case. Pass a count of zero or more.

Learn: https://desireco.github.io/zhex/errors.html#text-repeat-count

## text-to-int

The text given to `text.to-int` is not a signed 128-bit decimal integer.

Fix: It accepts only digits, with an optional sign; surrounding whitespace is ignored.

Learn: https://desireco.github.io/zhex/errors.html#text-to-int

## time-parse

A time string is invalid.

Fix: Use `HH:MM:SS`, optionally followed by fractional seconds.

Learn: https://desireco.github.io/zhex/errors.html#time-parse

## timezone-load

A named time zone is invalid or unavailable.

Fix: Use UTC or an IANA time zone name.

Learn: https://desireco.github.io/zhex/errors.html#timezone-load

## try-input

Postfix `try` and `catch` need a `Result` value to unwrap.

Fix: Results come from fallible words and operations. Inside a word, propagation requires a declared Result with the same error type.

Learn: https://desireco.github.io/zhex/errors.html#try-input

## unary-type

The unary operator (`-`, `not`) received the wrong type.

Fix: `-` needs `Int`; `not` needs `Bool`. When the checker can trace the value to the word that produced it, the message says where it comes from.

Learn: https://desireco.github.io/zhex/errors.html#unary-type

## unclosed-delimiter

An opening parenthesis, bracket, or brace has no matching closing delimiter.

Learn: https://desireco.github.io/zhex/errors.html#unclosed-delimiter

## unexpected-character

A character cannot start any token.

Fix: Check for stray punctuation or a non-ASCII symbol; names are ASCII letters, digits, underscores, and hyphenated segments.

Learn: https://desireco.github.io/zhex/errors.html#unexpected-character

## unhandled-result

Handle or propagate each Result with `catch` or `try`.

Fix: An immutable stored Result must be handled by the immediately following statement in the same block.

Learn: https://desireco.github.io/zhex/errors.html#unhandled-result

## unknown-name

No binding or word has this name.

Fix: The hint suggests the closest known one; if the name contains a hyphen, that hyphen is part of the name — subtraction is written `a - b` with spaces.

Learn: https://desireco.github.io/zhex/errors.html#unknown-name

## unknown-type

A type name is not one of `Int`, `Bool`, `String`, `Unit`, `Buffer`, or `File`.

Fix: Check the spelling; there are no user-defined types yet.

Learn: https://desireco.github.io/zhex/errors.html#unknown-type

## unknown-word

No word with this name exists.

Fix: The hint suggests the closest known word; check the spelling or define it.

Learn: https://desireco.github.io/zhex/errors.html#unknown-word

## unsupported-code

An internal invariant failed: the checker approved something the evaluator cannot run.

Fix: This is a bug in Zhex, not in your program — please report the program that triggered it.

Learn: https://desireco.github.io/zhex/errors.html#unsupported-code

## unterminated-string

A string literal has no closing quote before the end of the line.

Fix: Close it on the same line; strings do not continue across newlines.

Learn: https://desireco.github.io/zhex/errors.html#unterminated-string

## use-cycle

Modules import each other in a circle; the message spells the files in the cycle.

Fix: A circle has no first file to check, so one `use` in the cycle must be removed.

Learn: https://desireco.github.io/zhex/errors.html#use-cycle

## use-location

A `use` appears inside a word body or another block.

Fix: `use` is a top-level statement; move it to the top level of the file.

Learn: https://desireco.github.io/zhex/errors.html#use-location

## use-missing

A `use` names a module that could not be loaded.

Fix: The message names the module and the searched path — the sibling `<name>.zhex` next to the importing file. Create that file or fix the module name in the `use`.

Learn: https://desireco.github.io/zhex/errors.html#use-missing

## use-reserved

A module is named `text`, `int`, `buffer`, `file`, or `console`.

Fix: Those prefixes belong to the standard operations, so a module cannot take any of them. Rename the module file.

Learn: https://desireco.github.io/zhex/errors.html#use-reserved

## with-location

A `with` line did not directly follow the expression line it continues.

Fix: Put `with` on the next line below the seed expression, at the same indentation, with no blank line between. In the REPL, write the explicit `flow` form — `flow` on the seed line with `with` directly below it — because the REPL submits a bare seed line as its own entry.

Learn: https://desireco.github.io/zhex/errors.html#with-location

## with-seed

A `with` block's seed produces Unit, so no step can receive it.

Fix: Give `with` a value-producing expression line.

Learn: https://desireco.github.io/zhex/errors.html#with-seed

## with-step

A `with` step name resolved neither through the block's namespace nor as a word in scope.

Fix: Spell the step as a qualified name, or define the word it names.

Learn: https://desireco.github.io/zhex/errors.html#with-step

## word-result

The word's final statement produces a different type than its declared result.

Fix: Make the last statement's type match the signature, or fix the signature. A word with no declared result whose last line is an `if` or `match` with arms of different types fails the same way: if the word returns nothing, declare `-> Unit`; otherwise give both arms the same type.

Learn: https://desireco.github.io/zhex/errors.html#word-result

## xml-depth

XML nesting exceeds the supported depth of 256.

Fix: Reduce the nesting.

Learn: https://desireco.github.io/zhex/errors.html#xml-depth

## xml-field

An XML element or attribute name is unknown, duplicated, missing, collides with its counterpart, or is namespace-qualified.

Fix: Inspect the path and reason.

Learn: https://desireco.github.io/zhex/errors.html#xml-field

## xml-range

An XML integer or document exceeds the supported range.

Fix: Correct the value or reduce the element count.

Learn: https://desireco.github.io/zhex/errors.html#xml-range

## xml-syntax

XML text is invalid, carries a DTD or entity declaration, uses an undefined entity, or has more than one root element.

Fix: Correct the input syntax.

Learn: https://desireco.github.io/zhex/errors.html#xml-syntax

## xml-type

An XML element or attribute does not match the expected Zhex type, or mixes text with child elements.

Fix: Inspect the path and correct the value.

Learn: https://desireco.github.io/zhex/errors.html#xml-type

