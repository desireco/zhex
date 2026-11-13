---
title: Results and error handling
description: Declare fallible words, propagate or recover Result failures, and inspect structured errors.
section: Language
order: 50
---

Operations that can fail return a `Result` containing either a success value or an error. A word declares that possibility in its result type. `Result<T, E>` names a success type `T` and an error family `E`:

```zhex
word parse-count(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
```

`ok(value)` constructs the success case. `err(error)` constructs the failure case inside a fallible word. The supported error families are `InputError`, `ParseError`, `FileError`, `BufferError`, and the common `Error` family. User-defined error types are not part of the language.

The built-ins that return results include `read-line`, `text.to-int`, `console.arg`, file and buffer operations, `error.cause`, and checked list/array operations. See [console](console.html), [files](files.html), [buffers](buffers.html), and [lists and arrays](collections.html) for their success types and failure cases.

## Propagate with `try`

Postfix `try` unwraps success. On failure, it returns the error to the caller of the current fallible word. The error family must match that word's declared family; it is not silently converted.

```zhex
word parse-count(raw: String) -> Result<Int, ParseError>
    let count = text.to-int(raw) try
    ok(count)

word load-count(raw: String) -> Result<Int, ParseError>
    ok(parse-count(raw) try)

let count = load-count("12") try
print(count)
```

At the program boundary, a top-level `try` keeps the existing behavior: an unhandled error prints the source-located diagnostic and the run exits nonzero. A non-fallible word cannot use `try` to leak an error to the runner; it must recover it with `catch` or declare a compatible `Result` return type.

Flows follow the same rule. Mark a fallible flow step with `try`:

```zhex
word parse-count(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)

flow "12"
    parse-count try
    print
```

A bare fallible step is a checker error. A flow step marked `try` propagates failure through the current fallible word or to the top-level runner.

## Recover with `catch`

Write `catch` after a Result-producing expression as a statement, optionally naming the error. Its indented body runs only on failure and must produce the success type `T`:

```zhex
let count = text.to-int("bad") catch problem
    print(error.code(problem))
    print("Using the default count.")
    3
print(count)
```

On success, the handler does not run and the expression produces the parsed integer. On failure, `problem` is available only in the handler and the last statement supplies the replacement integer. An `if` handler needs compatible branch results. `catch` handles returned Result failures; it does not catch runtime faults such as integer overflow or division by zero, including a runtime fault while evaluating the protected expression.

If a handler needs to preserve the failure instead of returning a fallback, a fallible word can leave the handler early with `return err(error)`:

```zhex
word read-settings(path: String) -> Result<String, Error>
    let contents = file.read-all(path) catch problem
        return err(error.wrap("settings-read", "could not read settings", problem))
    ok(contents)
```

`error.wrap` creates a common `Error` and keeps the original error as its cause and source location. For a cause-less domain error, use `error.new(code, message)`. `return err(...)` is only valid inside a word whose declared result accepts that error family.

## Results must be handled immediately

A Result cannot be silently discarded. A bare fallible call is a checker error, even if its value is not used. If you store one with `let`, the next statement in that same block must handle that binding directly with `try` or `catch`:

```zhex
let pending = text.to-int("12")
let count = pending try
print(count)
```

The Result binding is immutable and cannot be aliased, assigned to a mutable variable, passed to another word, or handled conditionally or in a later block. The next statement must be exactly `pending try` or `pending catch ...`, either as a standalone expression or as the entire right-hand side of a new `let`. For example, `print(pending try)` is rejected. For shorter code, put `try` or `catch` directly on the call. A REPL entry follows the same rule.

Error families do not widen implicitly. A caught concrete-family error cannot be passed to a user-defined word that accepts `Error`, returned as the `Error` payload of `Result<T, Error>`, stored in an `Error` field/collection, or returned through an aggregate helper. Use `error.wrap(code, message, cause)` to convert it while preserving the cause. Pass the wrapped value directly in a named call; a flow cannot pass an Error value through a user-defined step, even when wrapped. Built-in `error.*` accessors, `error.report`, and `error.wrap` accept the caught error directly. Error bindings are immutable.

The [error guide](diagnostics.html) documents error inspection, causes, and the optional reporter.

## Built-ins that return results

The built-in operations that return results are:

| Operation | Result type | Example failure |
| --- | --- | --- |
| `read-line(prompt)` | `Result<String, InputError>` | End of input, invalid UTF-8, or input error. |
| `text.to-int(text)` | `Result<Int, ParseError>` | Input is not a signed 64-bit decimal integer. |
| `console.arg(i)` | `Result<String, InputError>` | Argument index is outside the supplied arguments. |
| `file.read-all(path)` | `Result<String, FileError>` | File cannot be opened or is invalid UTF-8. |
| `file.read-lines(path)` | `Result<Buffer, FileError>` | File cannot be opened or is invalid UTF-8. |
| `file.open(path)` | `Result<File, FileError>` | File cannot be opened. |
| `file.read-line(file)` | `Result<String, FileError>` | End of file, closed file, or read failure. |
| `buffer.line(buffer, i)` | `Result<String, BufferError>` | Index is outside the buffer's lines. |
| `buffer.set-line(buffer, i, line)` | `Result<Unit, BufferError>` | Index is outside the buffer's lines. |
| `error.cause(error)` | `Result<Error, Error>` | The error has no cause (`error-no-cause`). |
| `list.append(list, value)` | `Result<Unit, Error>` | The collection is being iterated (`collection-iterating`) or the size limit is exceeded (`collection-limit`). |
| `list[index]` | `Result<T, Error>` | Index is outside the list (`list-range`). |
| `(list[index] = value)` | `Result<Unit, Error>` | Invalid index or an active traversal prevents mutation. |
| `array.new(dimensions, fill)` | `Result<Array<T>, Error>` | Invalid rank/shape, or the element limit is exceeded. |
| `array.resize(array, dimensions, fill)` | `Result<Unit, Error>` | Invalid shape, limit exceeded, or an active traversal prevents mutation. |
| `array[i, j, ...]` | `Result<T, Error>` | Coordinates are outside the array (`array-range`). |
| `(array[i, j, ...] = value)` | `Result<Unit, Error>` | Invalid coordinates or an active traversal prevents mutation. |

See the [diagnostic guide](diagnostics.html) for structured error fields and the reporter, and the [error catalog](../errors.html) for stable diagnostic codes.
