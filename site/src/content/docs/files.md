---
title: Files
description: Read UTF-8 files all at once or stream their lines with automatic loop cleanup.
section: Standard library
order: 90
---

File operations read UTF-8 text. Choose a whole-file operation when the full content is needed, or stream lines when processing them one by one.

## Whole-file reads

| Signature | Result and behavior |
| --- | --- |
| `file.read-all(String) -> Result<String, FileError>` | On success, the file contents. |
| `file.read-lines(String) -> Result<Buffer, FileError>` | On success, a buffer containing chomped lines. |

Both operations return a result. Use postfix `try` to propagate a failure or `catch` to recover from one:

```zhex
let content = file.read-all("examples/text-stats-sample.txt") try
print(text.word-count(content))
```

Run that program from the repository root so the sample path resolves.

## Streaming reads

| Signature | Behavior |
| --- | --- |
| `file.open(String) -> Result<File, FileError>` | On success, open the file. |
| `file.read-line(File) -> Result<String, FileError>` | On success, return the next line. End of file is a failure. |
| `file.close(File) -> Unit` | Close the file. Calling it more than once is safe. |

These signatures show built-in return values. User words declare a success type; when a `try` can escape, the checker infers the word's effective `Result<T, E>` signature. Explicit Result signatures remain supported; see [results and error handling](results.html).

An open `File` is a resource: it cannot be printed or compared. Close a file after you finish reading. Reading from a closed file fails.

For a line-at-a-time loop, use `for line in file.lines(path)`. The loop streams chomped lines and always closes the file when it ends, including when its body fails:

```zhex
for line in file.lines("examples/text-stats-sample.txt")
    print(text.upper(line))
```

The `file.lines` form is available only as the source of a `for` loop.

## Lines and failures

Line reading removes one trailing line feed (`LF`) and removes an immediately preceding carriage return (`CR`) with it. A bare final `CR` without `LF` is retained. A final nonempty line without a newline is still read. An empty file produces no lines in a loop.

Opening or reading can fail, and invalid UTF-8 is rejected. Whole-file open failures use `file-open`; streaming read failures and end-of-file use `file-read`; invalid text uses `file-utf8`. Add `try` to propagate a failure or `catch` to handle it locally. See [results and error handling](results.html).

See the [text-stats example](../examples.html#text-stats), which counts a whole file or streams its lines depending on the supplied arguments.
