---
title: Results and try
description: Understand built-in operations that can fail and how postfix try propagates their errors.
section: Language
order: 50
---

Some built-in operations can fail. They return a result that contains either a success value or an error. Add postfix `try` to unwrap success; if the operation failed, `try` stops the current run with a diagnostic.

```zhex
let line = read-line("Number: ") try
let number = text.to-int(line) try
print(number * number)
```

`read-line` can fail at end of input or on invalid UTF-8 or an input error. `text.to-int` can fail when its input is not a signed 64-bit decimal integer. Neither failure silently becomes a default value.

## What `try` does

- On success, `try` produces the operation's value so it can be assigned, passed to a word, or used in an expression.
- On failure, `try` propagates the error to the runner. The run exits nonzero after writing a source-located diagnostic.
- A program is type-checked before it runs. A type error therefore occurs before any prompt, output, or input consumption.

Without `try`, a result is still a result. The checker rejects using it where an ordinary `String`, `Int`, or other value is expected. A result cannot be printed or compared.

## Built-ins that return results

The current result-producing operations are:

| Operation | Success value | Example failure |
| --- | --- | --- |
| `read-line(prompt)` | `String` | End of input, invalid UTF-8, or input error. |
| `text.to-int(text)` | `Int` | Input is not a signed 64-bit decimal integer. |
| `console.arg(i)` | `String` | Argument index is outside the supplied arguments. |
| `file.read-all(path)` | `String` | File cannot be opened or is invalid UTF-8. |
| `file.read-lines(path)` | `Buffer` | File cannot be opened or is invalid UTF-8. |
| `file.open(path)` | `File` | File cannot be opened. |
| `file.read-line(file)` | `String` | End of file, closed file, or read failure. |
| `buffer.line(buffer, i)` | `String` | Index is outside the buffer's lines. |
| `buffer.set-line(buffer, i, line)` | `Unit` | Index is outside the buffer's lines. |

`try` is the current error-handling mechanism. User-defined result types, recovery branches, and exception-catching syntax are not part of the language yet. See the [diagnostic guide](diagnostics.html) for checker hints and the [error catalog](../errors.html) for stable error codes.
