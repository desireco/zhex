---
title: Results and error handling
description: Infer or declare fallible word signatures, propagate or recover Result failures, and inspect structured errors.
section: Language
order: 50
---

Built-in operations that can fail return a `Result` containing either a success value or an error. User word parameters always have types; the return annotation may be omitted when the body determines one concrete success type. When a `try` can let a built-in error escape, the checker infers the word's effective `Result<T, E>` signature. A pure word keeps its inferred or declared success type. Successful values are wrapped once at the word boundary, so ordinary words do not need `ok(...)` around their final value.

```zhex
word parse-count(raw: String)
    flow raw
        text.trim
        text.to-int try

flow " 12 "
    parse-count try
    print
```

This prints `12`. The body infers the success type `Int`; `text.to-int try` can fail with `ParseError`, so callers see `parse-count(raw: String) -> Result<Int, ParseError>` and must use `try` or `catch`. If a body does not determine one concrete success type—for example, an unconstrained recursive cycle or an empty list result—add a return annotation.

The supported built-in error families are `InputError`, `ParseError`, `FileError`, `BufferError`, and the common `Error` family. User-defined error types are not part of the language. Input and parsing built-ins keep their explicit `Result<T, E>` signatures; selected collection operations offer checked forms.

The built-ins that return results include `read-line`, `text.to-int`, `console.arg`, file and buffer operations, `error.cause`, and checked list/array operations. See [console](console.html), [files](files.html), [buffers](buffers.html), and [lists and arrays](collections.html) for their success types and failure cases.

## Propagate with `try`

Postfix `try` unwraps success. On failure, it returns the error to the caller. The checker determines the escaping error family for a user word and includes it in that word's effective signature. Each caller still marks a fallible call with `try`, whether it uses a direct call or a flow step.

```zhex
word load-count(raw: String) -> Int
    parse-count(raw) try

print(load-count("12") try)
```

Using `parse-count` from the example above, `load-count` also has effective signature `Result<Int, ParseError>`. At the program boundary, a top-level `try` keeps the existing behavior: an unhandled error prints the source-located diagnostic and the run exits nonzero.

Forward calls, recursion, and imported words contribute their escaping effects to the effective signature. If uncaught `try` expressions can propagate more than one error family from a word, the checker rejects it; handle a failure with `catch` or convert it explicitly with `error.wrap` to the common `Error` family. Error families never widen implicitly. Runtime faults such as division by zero are not inferred as Result effects.

```zhex
word read-or-default(raw: String) -> Int
    text.to-int(raw) catch
        3

flow read-or-default("bad")
    print
```

A word that lets both `read-line` and `text.to-int` fail through `try` is rejected because they can propagate `InputError` and `ParseError`. In `read-or-default`, the `catch` handles the parse failure, so the word remains a plain `Int` operation. If a handler itself contains an escaping `try`, that error family can still be inferred.

An explicit `Result<T, E>` result type remains available when code needs to construct a result directly. `ok(value)` and `err(error)` construct that explicit result; the success is not wrapped a second time:

```zhex
word parse-explicit(raw: String) -> Result<Int, ParseError>
    ok(text.to-int(raw) try)
```

This form remains compatible with inferred effects. It also allows a word to make its Result construction visible at the declaration and return points.

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

Inside a word, a final `catch` expression can supply the word's success value directly:

```zhex
word retry-count(raw: String) -> Int
    text.to-int(raw) catch
        3

flow retry-count("bad")
    print
```

The word stays a plain `Int` operation because it handles the parse failure locally. The `flow` sends the fallback value straight to `print`.

If a handler needs to preserve the failure instead of returning a fallback, a fallible word can leave the handler early with `return err(error)`:

```zhex
word read-settings(path: String) -> String
    file.read-all(path) catch problem
        return err(error.wrap("settings-read", "could not read settings", problem))
```

`error.wrap` creates a common `Error` and keeps the original error as its cause and source location. For a cause-less domain error, use `error.new(code, message)`. The checker infers this word's effective signature as `Result<String, Error>`; `return err(...)` exits with an explicit error while an ordinary successful `String` is wrapped once at the word boundary.

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
