---
title: Console input and output
description: Print values, read lines, access program arguments, and choose an exit outcome.
section: Standard library
order: 100
---

`print(Int | Bool | String) -> Unit` writes one value followed by a newline; booleans print as `true` or `false`. This signature notation lists the supported overloads; `|` is not Zhex syntax.

`read-line(String) -> Result<String, InputError>` writes and flushes its prompt, then reads one line from standard input. Use `try` to propagate failure or `catch` to choose a fallback:

```zhex
let name = read-line("Name: ") try
print("Hello, " + name + "!")
```

The line keeps its spaces, and an empty line is valid. One final `LF`, and an immediately preceding `CR`, are removed. A final line without a newline is accepted. End of input before a line, invalid UTF-8, and input errors fail; they do not silently become an empty string.

## Program arguments

Arguments are the words after the source filename in the `run` command:

```sh
zhex run examples/text-stats.zhex examples/text-stats-sample.txt
```

In this invocation, `console.args-count() -> Int` returns `1`, and `console.arg(0) -> Result<String, InputError>` returns a result containing the sample file path. Use `try` with `console.arg`; a negative or out-of-range index fails with `argument-missing`.

These signatures show built-in return values. User words can omit a return annotation when the body determines one concrete success type; an escaping `try` also makes the checker infer the effective Result signature. Explicit return and `Result<T, E>` signatures remain supported; see [Results and error handling](results.html).

```zhex
let path = console.arg 0 try
print path
```

Extra positional arguments are forwarded to the program. Command options such as `--trace` and `--report=json` belong to the interpreter and are not program arguments. Unknown options are rejected.

Static checker errors occur before any input is read or output is written. Runtime failures can happen after earlier output, such as a prompt, has already appeared. The `zhex` executable exits `0` after a successful run, `1` for file or language errors, and `2` for invalid command usage.

## End a program with a chosen status

`console.exit(status)` terminates the program immediately with an integer status from `0` through `255`. A valid deliberate exit writes no diagnostic and does not trigger the automatic error reporter. Status `0` means success; a nonzero status lets the program report failure in its own way. It may appear in a `catch` handler, where it ends the run instead of supplying the Result's fallback value:

```zhex
let line = read-line("Number: ") try
let number = text.to-int(line) catch problem
    print("Invalid number: " + error.code(problem))
    console.exit(2)
print(number)
```

If `status` is outside `0..255`, `console.exit` raises a source-located `console-exit-status` runtime diagnostic instead of clamping or wrapping it. That runtime failure is eligible for the configured error reporter. Active `file.lines` streams close on exit; files opened explicitly with `file.open` remain the program's responsibility to close with `file.close`. In the REPL, `console.exit` ends the entire process with the requested status.

## Fail with a message

`console.fail(message)` writes the message and a newline to standard error, then ends the program with status `1`. It never returns, so it can be used as a typed `catch` fallback:

```zhex
let name = read-line("Name: ") catch problem
    console.fail("Could not read a name: " + error.code(problem))
print("Hello, " + name)
```

This is a deliberate failure path: it writes no language diagnostic and does not call the automatic error reporter. Active `file.lines` streams close as the program exits; explicit `file.open` handles still need `file.close`. Calling `console.fail` from a reporter cannot replace the original program outcome.
