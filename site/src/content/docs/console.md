---
title: Console input and output
description: Print values, read lines, and access arguments passed to a Zhex program.
section: Standard library
order: 100
---

`print(Int | Bool | String) -> Unit` writes one value followed by a newline; booleans print as `true` or `false`. This signature notation lists the supported overloads; `|` is not Zhex syntax.

`read-line(String) -> Result<String, InputError>` writes and flushes its prompt, then reads one line from standard input. Its result must be unwrapped with `try`:

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

Result signatures here describe built-in return values; programs cannot write `Result<T, E>` in a word signature.

```zhex
let path = console.arg(0) try
print(path)
```

Extra positional arguments are forwarded to the program. Command options such as `--trace` and `--report=json` belong to the interpreter and are not program arguments. Unknown options are rejected.

Static checker errors occur before any input is read or output is written. Runtime failures can happen after earlier output, such as a prompt, has already appeared. The `zhex` executable exits `0` after a successful run, `1` for file or language errors, and `2` for invalid command usage.
