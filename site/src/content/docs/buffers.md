---
title: Buffers
description: Build and edit line-oriented text with mutable Buffer values.
section: Standard library
order: 80
---

A `Buffer` is a growable sequence of `String` lines. It is useful when a program needs to build or edit text one line at a time.

```zhex
let lines = buffer.new()
buffer.add-line(lines, "first")
buffer.add-line(lines, "second")
buffer.set-line(lines, 1, "updated") try
print(buffer.to-text(lines))
```

This prints:

```text
first
updated
```

## Operations

| Signature | Result and behavior |
| --- | --- |
| `buffer.new() -> Buffer` | Create an empty buffer. |
| `buffer.add-line(Buffer, String) -> Unit` | Append one line. |
| `buffer.line-count(Buffer) -> Int` | Return the number of lines. |
| `buffer.line(Buffer, Int) -> Result<String, BufferError>` | On success, return the line at a zero-based index. |
| `buffer.set-line(Buffer, Int, String) -> Result<Unit, BufferError>` | On success, replace the line at a zero-based index. |
| `buffer.to-text(Buffer) -> String` | Join the lines with `\n`. |

`buffer.line` and `buffer.set-line` fail when the index is negative or is at least the line count. Add `try` to either call to propagate that failure. `Result<T, E>` here describes the built-in operation result; it cannot be written in a word signature.

Names refer to the same buffer value; binding it with `let` does not make a copy. A second name observes changes made through the first. Buffers cannot be compared with `==` or `!=`, and `print` does not accept a `Buffer`; convert it with `buffer.to-text` first.

For whole-file input as a buffer, see [`file.read-lines`](files.html). For line and word counts, see [text operations](text.html).
