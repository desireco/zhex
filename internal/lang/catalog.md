# Diagnostic catalog

Every Zhex diagnostic carries one of these stable codes. The first line of a
diagnostic is always `file:line:column: message`; the source excerpt, caret,
and `hint:` line are additive detail. Machine readers use
`zhex run <file> --report=json`.

## list-range
The list index is negative or outside its current length. Use a valid zero-based index.

## collection-limit
A list or array would exceed the runtime limit of 1,000,000 elements. Reduce its size.

## collection-iterating
An active iteration prevents mutation of the traversed collection. Mutate a different collection or wait until traversal finishes.

## console-exit-status
`console.exit` requires a status from 0 through 255. Choose a portable exit status in that range.

## unhandled-result
Handle or propagate each Result with `catch` or `try`. An immutable stored Result must be handled by the immediately following statement in the same block.

## error-no-cause
`error.cause` was called for an Error without a cause. Check `error.has-cause` first or handle the Result.

## source-utf8
The file is not valid UTF-8. Re-save it as UTF-8; Zhex source has no other encoding.

## tab-character
A tab appeared outside a string. Indentation is four spaces per level; replace tabs with spaces.

## indentation
A line's indent is not a multiple of four spaces, or it opens more than one level at once. Each block indents exactly four spaces from its parent.

## unterminated-string
A string literal has no closing quote before the end of the line. Close it on the same line; strings do not continue across newlines.

## string-escape
A string contains an escape Zhex does not support, or invalid UTF-8. Strings are double-quoted and use Go-style escapes like `\n` and `\"`.

## unexpected-character
A character cannot start any token. Check for stray punctuation or a non-ASCII symbol; names are ASCII letters, digits, underscores, and hyphenated segments.

## expected-token
The parser expected a specific token and found something else. The message names both; fix the line to match the expected grammar.

## expected-name
A keyword appears where a name is required. `word`, `let`, `var`, `if`, and the other keywords are reserved; choose a name that is not one of them.

## unknown-type
A type name is not one of `Int`, `Bool`, `String`, `Unit`, `Buffer`, or `File`. Check the spelling; there are no user-defined types yet.

## expected-indent
A construct needs an indented block on the following lines. Indent the body four spaces. Open `()`, `[]`, and `{}` delimiters can continue expressions across lines, but do not replace a required block body.

## empty-block
A block header is followed by nothing. Every block needs at least one statement.

## nesting-limit
Expressions or blocks nest past the parser's limit. Split the expression into named parts with `let`.

## integer-literal-range
An integer literal is outside the signed 64-bit range. Keep literals within ±9223372036854775807; compute larger values at runtime.

## expected-expression
A token cannot start an expression where one is required.

## flow-step
A `flow` block needs at least one step word. Write one word per line; the first receives the seed value.

## unknown-name
No binding or word has this name. The hint suggests the closest known one; if the name contains a hyphen, that hyphen is part of the name — subtraction is written `a - b` with spaces.

## duplicate-name
This name is already defined in the same scope. Choose a fresh name; shadowing is not allowed.

## duplicate-word
A word with this name already exists, or it collides with a host word. Rename one of them.

## word-result
The word's final statement produces a different type than its declared result. Make the last statement's type match the signature, or fix the signature.

## immutable-name
A name defined with `let` cannot be reassigned. Declare it with `var` to allow updates.

## assignment-type
A binding annotation must match its value's type.
The new value's type differs from the variable's type. There are no implicit conversions; convert explicitly (for example `int.to-string`).
When the checker can trace the value to the word that produced it, the message says where it comes from.

## condition-type
An `if` condition must be `Bool`. Comparisons return `Bool`; integers and strings do not.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## branch-type
The branches of an `if` end in different types. Make every branch produce the same type.

## range-type
The bounds of `range(start, end)` must both be `Int`; the path of `file.stream(path)` must be `String`.

## try-input
Postfix `try` and `catch` need a `Result` value to unwrap. Results come from fallible words and operations. Inside a word, propagation requires a declared Result with the same error type.

## unary-type
The unary operator (`-`, `not`) received the wrong type. `-` needs `Int`; `not` needs `Bool`.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## operand-types
A binary operator got two different operand types. If one side is a `Result`, add postfix `try` to unwrap it.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## bool-operand
`and`/`or` need `Bool` on both sides. Comparisons produce `Bool`; integers do not.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## compare-type
Equality comparison needs matching `Int`, `Bool`, or `String` values. If one side is a `Result`, add postfix `try` to unwrap it.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## ordering-type
Ordering comparison (`< <= > >=`) needs `Int` operands.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## addition-type
`+` adds two `Int`s, concatenates two `String`s, or concatenates a `String` and an `Int` — the string-context Int exception. Other equal operand types, such as two `Bool`s, cannot use `+`; convert explicitly.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## arithmetic-type
`- * / %` need `Int` operands.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## argument-count
A word call passed the wrong number of arguments. The message states how many the word takes.

## argument-type
An argument's type does not match the parameter. There are no implicit conversions; convert at the call site.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## unknown-word
No word with this name exists. The hint suggests the closest known word; check the spelling or define it.

## print-type
`print` accepts one `Int`, `String`, or `Bool` value. If it received a `Result`, add postfix `try` to unwrap it.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## range-context
`range` and `file.stream` exist only inside a `for` loop header — `for n in range(start, end)` and `for line in file.stream(path)`. They cannot be called elsewhere.

## use-missing
A `use` names a module that could not be loaded. The message names the module and the searched path — the sibling `<name>.zhex` next to the importing file. Create that file or fix the module name in the `use`.

## use-cycle
Modules import each other in a circle; the message spells the files in the cycle. A circle has no first file to check, so one `use` in the cycle must be removed.

## use-reserved
A module is named `text`, `int`, `buffer`, `file`, or `console`. Those prefixes belong to the standard operations, so a module cannot take any of them. Rename the module file.

## use-location
A `use` appears inside a word body or another block. `use` is a top-level statement; move it to the top level of the file.

## module-body
An imported file contains top-level statements. Imported files declare words only; move the statements into the file you run.

## private-use
A word or record type beginning with an underscore is private to its module. Public declarations cannot expose a private record type. Make the declaration private or use a public type.

## integer-overflow
An arithmetic result does not fit in a signed 64-bit integer. Overflow is an error in Zhex, not a wraparound; keep intermediate values in range.

## division-zero
Division or remainder by zero. Guard the divisor with a condition first.

## depth-limit
Word recursion passed the runtime depth limit of 256. Rewrite the recursion as a loop with `var` and `for`; loops have no depth limit.

## unsupported-code
An internal invariant failed: the checker approved something the evaluator cannot run. This is a bug in Zhex, not in your program — please report the program that triggered it.

## console-output
Writing to the output stream failed. Check that the output is writable (closed pipes fail this way).

## console-input
Reading a line from the input stream failed, usually because the input ended. The `reason` detail is `end-of-file` when the input ended (Ctrl-D or piped EOF; an empty line is a success, not this error) and `read-failed` for a read failure; read it with `error.detail` to tell them apart. The failure propagates through `try` and exits the program; check that input is available.

## console-utf8
The line read from the input stream is not valid UTF-8. Zhex reads UTF-8 text; supply the input as UTF-8.

## text-to-int
The text given to `text.to-int` is not a signed 64-bit decimal integer. It accepts only digits, with an optional sign; surrounding whitespace is ignored.

## replace-empty
`text.replace` was called with an empty old text. Every position matches an empty string, so there is no sane result; this is a bug signal, not a recoverable case. Pass a nonempty old text.

## buffer-range
`buffer.line` or `buffer.set-line` received an index outside the buffer's lines. The failure lives inside the `Result` and surfaces through `try`; check the index against `buffer.line-count` first.

## file-open
`file.open`, `file.read`, or a `file.stream` loop could not open the path. The message names the path and the operating system's reason; check the path and its permissions.

## file-read
A read from an open file failed: the stream was drained ("end of file") or the file was closed. `file.read-line` at end of file fails inside the `Result`; to read a whole file, prefer `file.read(path)` or `for line in file.stream(path)`.

## file-utf8
A file's content is not valid UTF-8. Zhex reads UTF-8 text only; re-save the file as UTF-8.

## file-write
`file.write` could not open, write, or close the path; the message names the path, the failing phase, and the operating system's reason. The failure lives inside the `Result` and surfaces through `try`; a failed write may leave a partial or truncated file, and a missing parent directory is never created.

## file-exists
`file.exists` could not answer for the path: the message names the path and the operating system's reason, and the failure carries the not-found/permission-denied/other taxonomy in its details. A missing path is not this failure — it is the answer `false`; apply `try` or `catch` to inspect the checked form's failures.

## file-list
`file.list` could not list the folder: the folder is missing, or the process may not read it, or the path is a file. The message names the folder and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

## file-size
`file.size` could not get the size of the path: it is missing, or the process may not stat it. The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

## file-free-space
`file.free-space` could not report the bytes available on the volume containing the path: the path does not exist, or the platform cannot answer for it. The message names the path and the reason; the failure lives inside the `Result` and surfaces through `try`.

## file-make-folder
`file.make-folder` could not create the folder: a parent is missing (pass `parents: true` to create ancestors), or the target already exists (the default one-level form refuses it; `parents: true` tolerates an existing folder), or the process may not write the parent. The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

## file-delete
`file.delete` could not delete the path: it is missing, it is a nonempty folder (the safety default removes a file or an empty folder only; `file.delete-folder(recursive: true)` removes a tree), or the process may not remove it. The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

## file-delete-folder
`file.delete-folder` could not remove the folder: it holds entries while `recursive` is false (the default removes an empty folder only; pass `recursive: true` for the whole tree), or the process may not remove something inside it. Recursive removal is best-effort: part of the tree may already be gone when the first failure is reported, and earlier effects stand. The message names the folder and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

## file-move
`file.move` could not rename the source to the target: the source is missing, the target exists (pass `overwrite: true` to replace an existing file), the target is a folder (never replaced in either mode), the two paths are on different devices (no copy fallback), or the process may not rename. The check-then-act window between the target check and the rename is inherent: a target that appears in between is left to the rename itself. The message names both paths and the reason; the failure lives inside the `Result` and surfaces through `try`.

## env-read
`env.load` could not read the file: it is missing, or the process may not read it. The message names the path and the operating system's reason; the failure lives inside the `Result` and surfaces through `try`.

## env-parse
A line of the file given to `env.load` is not a KEY=VALUE assignment: the `=` is missing, the key is empty, or the key contains spaces. The message names the line number and the offending text. Lines before the malformed one are already applied and stay applied — env.load never overrides, so rerunning after fixing the file keeps the earlier values.

## argument-missing
`console.arg` received an index outside the program's command-line arguments. The message names the index and how many arguments were given; the failure lives inside the `Result` and surfaces through `try`. Check the index against `console.args-count()`.

## array-range

An array coordinate is outside its dimension.

## array-rank

An array shape or index has the wrong rank.

## array-shape

An array extent is negative.

## json-syntax

JSON text is invalid or contains more than one value. Correct the input syntax.

## json-type

A JSON value does not match the expected Zhex type. Inspect the path and correct the value.

## json-field

A JSON object has an unknown, duplicate, or missing required field. Inspect the path and reason.

## json-range

A JSON integer or collection exceeds the supported range. Correct the value or reduce the list size.

## json-depth

JSON nesting exceeds the supported depth of 256. Reduce the nesting.

## xml-syntax

XML text is invalid, carries a DTD or entity declaration, uses an undefined entity, or has more than one root element. Correct the input syntax.

## xml-type

An XML element or attribute does not match the expected Zhex type, or mixes text with child elements. Inspect the path and correct the value.

## xml-field

An XML element or attribute name is unknown, duplicated, missing, collides with its counterpart, or is namespace-qualified. Inspect the path and reason.

## xml-range

An XML integer or document exceeds the supported range. Correct the value or reduce the element count.

## xml-depth

XML nesting exceeds the supported depth of 256. Reduce the nesting.

## date-parse

A date string is invalid. Use a Gregorian date in `YYYY-MM-DD` form.

## time-parse

A time string is invalid. Use `HH:MM:SS`, optionally followed by fractional seconds.

## duration-parse

A duration string is invalid or outside the supported nanosecond range.

## temporal-invalid

Numeric date or time components are outside their valid range.

## temporal-range

Temporal arithmetic produced a value outside the supported range.

## instant-parse

An instant string is invalid. Include a date, time, and explicit RFC 3339 offset.

## timezone-load

A named time zone is invalid or unavailable. Use UTC or an IANA time zone name.

## unclosed-delimiter

An opening parenthesis, bracket, or brace has no matching closing delimiter.

## doctest-mismatch

A `## >` example in a word's doc produced a different value than the expected output lines after it (they run until a blank doc line or the next `## >` line). Update the expected output, or fix the word. Checked by `zhex doctests <file>`; `zhex run` never executes docs.

## doctest-check

A `## >` example's expression failed to check or evaluate by itself. Fix the expression so it checks and evaluates — a naked fallible call needs `try`, and a Result cannot be echoed bare.

## test-failed

A `.test.zhex` file run by `zhex test` exited nonzero without a diagnostic of its own — a failed `test.check` label, a deliberate exit, or a crash. Run `zhex run <file>` to see the failing check in place.

## not-available-in-build

The build that ran the program does not provide this construct. `zhex compile` fences constructs its emitter does not support yet; the browser engine build has no filesystem or console arguments (`file.*` filesystem words, `file.stream`, `console.arg`, `env.load`). Run the program with `zhex run`, or rewrite it with constructs the build supports.
