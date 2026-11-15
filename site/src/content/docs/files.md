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
| `file.read(String) -> Result<String, FileError>` | On success, the file contents. |

The operation returns a result. Use postfix `try` to propagate a failure or `catch` to recover from one:

```zhex
let content = file.read("examples/text-stats-sample.txt") try
print(text.word-count(content))
```

Run that program from the repository root so the sample path resolves.

## Streaming reads

| Signature | Behavior |
| --- | --- |
| `file.open(String) -> Result<File, FileError>` | On success, open the file. |
| `file.read-line(File) -> Result<String, FileError>` | On success, return the next line. End of file is a failure. |
| `file.close(File) -> Unit` | Close the file. Calling it more than once is safe. |

These signatures show built-in return values. User words can omit a return annotation when the body determines one concrete success type; when a `try` can escape, the checker infers the word's effective `Result<T, E>` signature. Explicit return and Result signatures remain supported; see [results and error handling](results.html).

An open `File` is a resource: it cannot be printed or compared. Close a file after you finish reading. Reading from a closed file fails.

For a line-at-a-time loop, use `for line in file.stream(path)`. The loop streams chomped lines and always closes the file when it ends, including when its body fails:

```zhex
for line in file.stream("examples/text-stats-sample.txt")
    print(text.upper(line))
```

The `file.stream` form is available only as the source of a `for` loop.

## Writing a file

| Signature | Result and behavior |
| --- | --- |
| `file.write(String, String, Bool = false) -> Result<Unit, FileError>` | On success, `Unit`: the contents are written to the path. The optional third argument appends instead of truncating. |

`file.write(contents, path, append)` takes the contents first, then the path. Omitting `append` (or passing `false`) creates the file or truncates an existing one; `append = true` appends and creates the file if it is missing. The trailing `append` argument is a built-in constant default: calls may omit it, and a partial such as `file.write(_, "out.txt")` keeps it deferred, like a user-word default.

No parent directory is created, and the write is not atomic — a failure can leave a partial or truncated file. The open, write, and close happen inside the operation, so no writable file resource escapes. A created file uses mode 0644 subject to the process umask, and the write follows symlinks.

```zhex
let saved = file.write(report, "/") catch problem
    print("Save failed: " + error.code(problem))
```

## Inspecting paths

| Signature | Result and behavior |
| --- | --- |
| `file.list(String) -> Result<List<String>, FileError>` | On success, the folder's entry names. |
| `file.exists(String) -> Bool` | `true` when the path exists; `false` when it does not. |
| `file.size(String) -> Result<Int, FileError>` | On success, the size in bytes. |
| `file.free-space(String) -> Result<Int, FileError>` | On success, the bytes available on the volume containing the path. |

`file.list(folder)` lists one level: bare entry names, not paths, in sorted order, dot entries included — filtering them is `list.filter` work — as a fresh list. A folder with more than one million entries fails `collection-limit` inside the result before the list is built.

`file.exists(path)` is a plain `Bool`. A missing path is the answer `false`, not a failure; any other stat failure (a parent without permission, an invalid path) is a runtime diagnostic. Applying `try` or `catch` to the call selects the checked `Result<Bool, FileError>` form.

`file.size(path)` returns a regular file's size in bytes; a directory reports `0`, and nothing is summed recursively. `file.free-space(path)` accepts any existing path, file or folder, and reports the bytes available to the process on the volume containing it; it works on Unix and Windows, and other platforms fail `file-free-space` inside the result.

```zhex
let names = file.list("examples") try
print(names)
```

A listing that cannot be read fails `file-list`; a stat failure behind `file.exists` fails `file-exists`; size and free-space failures use `file-size` and `file-free-space`. Each names the path and the operating system's reason.

## Lines and failures

Line reading removes one trailing line feed (`LF`) and removes an immediately preceding carriage return (`CR`) with it. A bare final `CR` without `LF` is retained. A final nonempty line without a newline is still read. An empty file produces no lines in a loop.

Opening or reading can fail, and invalid UTF-8 is rejected. Whole-file open failures use `file-open`; streaming read failures and end-of-file use `file-read`; invalid text uses `file-utf8`. A write that cannot open, write, or close the path fails `file-write`, naming the path, the failing phase, and the operating system's reason; a failed write may leave a partial or truncated file, and a missing parent directory is never created. Add `try` to propagate a failure or `catch` to handle it locally. See [results and error handling](results.html).

See the [text-stats example](../examples.html#text-stats), which counts a whole file or streams its lines depending on the supplied arguments.
