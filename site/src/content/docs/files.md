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
| `file.write-bytes(String, List<Int>) -> Result<Unit, FileError>` | On success, `Unit`: the bytes are written to the path. Values must be 0 through 255. |

`file.write(contents, path, append)` takes the contents first, then the path. Omitting `append` (or passing `false`) creates the file or truncates an existing one; `append = true` appends and creates the file if it is missing. The trailing `append` argument is a built-in constant default: calls may omit it, and a partial such as `file.write(_, "out.txt")` keeps it deferred, like a user-word default.

No parent directory is created, and the write is not atomic — a failure can leave a partial or truncated file. The open, write, and close happen inside the operation, so no writable file resource escapes. A created file uses mode 0644 subject to the process umask, and the write follows symlinks.

`file.write-bytes(path, bytes)` is the binary counterpart: the path first, then a `List<Int>` holding one byte per `Int`. It always creates or truncates — there is no append mode — and it shares `file.write`'s parent-directory, mode, and phase rules. Every value must be 0 through 255; the first value outside that range fails `file-bytes-range` inside the `Result` before the file is opened, so a range failure never creates or truncates anything. There is no byte read and no byte type: the list is the byte representation. This is how binary formats start (a ZIP's `mimetype` entry, a fixed-width header), because `file.write` encodes its text as UTF-8 and a code point at or above U+0080 never lands as one byte.

```zhex
let saved = file.write-bytes("/tmp/mimetype", [109, 105, 109, 101, 116, 121, 112, 101]) catch problem
    print("Byte save failed: " + error.code(problem))
```

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

`file.exists(path)` is a plain `Bool`. A missing path is the answer `false`, not a failure; any other stat failure (a parent without permission, an invalid path) is a runtime diagnostic. Applying `try` or `catch` to the call selects the checked `Result<Bool, FileError>` form. The checked form exists only on the direct call: a deferred partial such as `let f = file.exists _` locks the plain form, and `f(path) try` is rejected.

`file.size(path)` returns a regular file's size in bytes; a directory reports `0`, and nothing is summed recursively. `file.free-space(path)` accepts any existing path, file or folder, and reports the bytes available to the process on the volume containing it; it works on Unix and Windows, and other platforms fail `file-free-space` inside the result.

```zhex
let names = file.list("examples") try
print(names)
```

A listing that cannot be read fails `file-list`; a stat failure behind `file.exists` fails `file-exists`; size and free-space failures use `file-size` and `file-free-space`. Each names the path and the operating system's reason.

## Composing and decomposing paths

| Signature | Result and behavior |
| --- | --- |
| `file.join(String, String) -> String` | The two paths joined with the native separator and cleaned. |
| `file.base(String) -> String` | The final path element. |
| `file.extension(String) -> String` | The final element's final-dot suffix, dot included. |
| `file.directory-of(String) -> String` | The path without its final element. |

These four words are pure lexical operations: no file is read, written, or checked, and every one returns a plain `String` — never a `Result` — so no `try` or `catch` applies. Separators follow the host platform.

`file.join(base, child)` concatenates and cleans: `file.join("a/b", "../c")` is `"a/c"`, and an empty side collapses to the other. An absolute child replaces the base — `file.join("a/b", "/c")` is `"/c"` — so an untrusted absolute component never nests under the base.

`file.base(path)` returns the final element with trailing separators removed; the root is its own base (`"/"`), and the empty path yields `"."`. `file.extension(path)` returns the suffix from the final element's final dot, including the dot: `".gz"` for `report.tar.gz`, `".bashrc"` for the dotfile `.bashrc`, and empty when there is none. `file.directory-of(path)` drops the final element; a path ending in a separator has no final element to drop (`"/a/b/"` yields `"/a/b"`), the root is its own directory, and a path with nothing to drop yields `"."`.

`..` passes through or cleans lexically — `file.join` resolves it during the clean, while `file.base` and `file.directory-of` treat it as an ordinary element. None of this is a containment guarantee.

```zhex
let invoice = file.join("/tmp/zhex-out", "june/invoices.csv")
print(file.directory-of(invoice))
print(file.base(invoice))
print(file.extension(invoice))
```

See the [paths example](../examples.html#paths), which composes report paths and decomposes them again without touching the disk.

## Creating, removing, and moving

| Signature | Result and behavior |
| --- | --- |
| `file.make-folder(String, Bool = false) -> Result<Unit, FileError>` | On success, `Unit`: the folder exists. The optional second argument creates missing ancestors. |
| `file.delete(String) -> Result<Unit, FileError>` | On success, `Unit`: a file or an empty folder was removed. |
| `file.delete-folder(String, Bool = false) -> Result<Unit, FileError>` | On success, `Unit`: an empty folder was removed; the optional second argument removes the whole tree. |
| `file.move(String, String, Bool = false) -> Result<Unit, FileError>` | On success, `Unit`: the source was renamed to the target, file or folder. The optional third argument replaces an existing target file. |

`file.make-folder(path, parents)` creates one level and fails on an existing target; `parents = true` creates every missing ancestor and tolerates a folder that already exists. The trailing mode arguments are built-in constant defaults, like `file.write`'s `append`: calls may omit them, and partials keep them deferred.

`file.delete(path)` removes a file or an empty folder and fails on a nonempty folder — the safety default. `file.delete-folder(path, recursive)` removes an empty folder by default; `recursive = true` removes the whole tree best-effort, so part of the tree may already be gone when the first failure is reported and earlier effects stand. Deleting a tree is always a deliberate, visible `recursive = true` at the call site.

`file.move(source, target, overwrite)` is rename-based, so it moves folders as well as files. With `overwrite` omitted or `false` it fails when the target exists; with `overwrite = true` it replaces an existing file only — an existing folder target fails under both values, and a cross-device move fails with the platform's error rather than copying. The check of the target and the rename have an unavoidable check-then-act window: a target that appears in between is left to the rename itself.

```zhex
file.make-folder("out/reports", true) try
file.move("draft.txt", "out/reports/report.txt") try
```

Failures use `file-make-folder`, `file-delete`, `file-delete-folder`, and `file-move`, naming the path or paths and the operating system's reason. Failures leave earlier effects in place — no transaction promise.

See the [file-work example](../examples.html#file-work), which builds a work folder, inspects it, and removes the whole tree.

## Lines and failures

Line reading removes one trailing line feed (`LF`) and removes an immediately preceding carriage return (`CR`) with it. A bare final `CR` without `LF` is retained. A final nonempty line without a newline is still read. An empty file produces no lines in a loop.

Opening or reading can fail, and invalid UTF-8 is rejected. Whole-file open failures use `file-open`; streaming read failures and end-of-file use `file-read`; invalid text uses `file-utf8`. A write that cannot open, write, or close the path fails `file-write`, naming the path, the failing phase, and the operating system's reason; a failed write may leave a partial or truncated file, and a missing parent directory is never created. `file.write-bytes` fails `file-write` the same way, and a byte value outside 0 through 255 fails `file-bytes-range` before the file is touched. Add `try` to propagate a failure or `catch` to handle it locally. See [results and error handling](results.html).

See the [text-stats example](../examples.html#text-stats), which counts a whole file or streams its lines depending on the supplied arguments.
