# Getting started

Build the interpreter, create a small Zhex program, and run it. You need Git and Go 1.23 or newer — both come from your system's package manager. For the shorter public tutorial, see [Getting started on the website](https://desireco.github.io/zhex/docs/getting-started.html).

## Install the tools

**Linux** — use your distribution's package manager:

```sh
sudo apt install git golang-go    # Debian, Ubuntu
sudo dnf install git golang       # Fedora
sudo pacman -S git go             # Arch
```

Check `go version`. If your distribution's Go is older than 1.23, install the current release from [go.dev/dl](https://go.dev/dl/) instead.

**macOS** — use [Homebrew](https://brew.sh/):

```sh
brew install git go
```

**Windows** — run PowerShell and use [winget](https://learn.microsoft.com/windows/package-manager/winget/):

```powershell
winget install Git.Git
winget install GoLang.Go
```

With [Scoop](https://scoop.sh/) or Chocolatey, `scoop install git go` or `choco install git golang` work too. Open a new terminal after installing so both tools are on your `PATH`. If you use WSL, follow the Linux steps unchanged inside a WSL shell.

## Run your first program

Clone the repository, build the interpreter once, and add it to this terminal's `PATH`.

**Linux and macOS:**

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
go build -o bin/zhex ./cmd/zhex
export PATH="$PWD/bin:$PATH"
```

**Windows (PowerShell):**

```powershell
git clone https://github.com/desireco/zhex.git
cd zhex
go build -o bin\zhex.exe .\cmd\zhex
$env:Path = "$PWD\bin;$env:Path"
```

The `PATH` change applies to the current terminal only. In a new terminal, run the last line again after changing to the repository.

To make the command permanent, install the binary into a bin folder with [just](https://just.systems/man/en/):

```sh
just install
```

The recipe builds `bin/zhex` and copies it to `~/.local/bin`, creating the folder when missing; pass any bin directory you choose to change the destination — system paths like `/usr/local/bin` need sudo. `~/.local/bin` must be on your `PATH` — if a new terminal cannot find `zhex`, add `export PATH="$HOME/.local/bin:$PATH"` to `~/.bashrc`, `~/.zshrc`, or your shell's profile and open a new terminal. The plain build plus `export PATH` above keeps working without `just`.

**Windows (PowerShell):** `just install` needs a POSIX shell; build as shown above, copy the executable, and put its folder on `Path`:

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\.local\bin" | Out-Null
Copy-Item bin\zhex.exe "$env:USERPROFILE\.local\bin\"
```

Add `%USERPROFILE%\.local\bin` to `Path` in "Edit the system environment variables" and open a new terminal.

Create `hello.zhex` in the repository root with this line, using any text editor — Notepad works:

```zhex
print("Hello, Zhex!")
```

Run it and check the output:

```sh
zhex run hello.zhex
```

```text
Hello, Zhex!
```

A program runs its top-level statements. A file whose top level has none may instead define `word main` with no parameters; `zhex run` and `zhex compile` invoke it automatically as the entry point.

Replace the file contents with a word that reads a name and sends it through a flow:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = read-line "Your name: " try

flow name
    greet
    print
```

Run `zhex run hello.zhex` again. Enter `Ada` at the prompt:

```text
Your name: Ada
Hello, Ada!
```

`greet` accepts and returns a `String`; its final expression is the result. `read-line` can fail, so `try` handles its `Result`. The flow sends the name to `greet`, then sends the greeting to `print`. The checker verifies word names, parameter types, and flow connections before execution.

For an included program, run `zhex run examples/hello.zhex`. Piped input is not echoed by the terminal:

```sh
printf 'Ada\n' | zhex run examples/hello.zhex
```

```powershell
'Ada' | zhex run examples\hello.zhex
```

```text
Your name: Hello, Ada!
```

## Try it interactively

Start a REPL session:

```sh
zhex repl
```

Words and values persist across entries. Parenthesized calls remain available; simple calls may also use comma-separated arguments at a complete expression boundary (`print 1 + 1` prints `2`). A word header opens a multi-line entry, shown with the `....> ` continuation prompt. In an interactive terminal, the editor pre-fills four spaces and carries indentation into nested blocks:

```text
Zhex REPL — end multi-line entries with a blank line; bye() or Ctrl-D exits.
zhex> word greet(name: String) -> String
....>     "Hello, " + name + "!"
....>
zhex> print(1 + 1)
2
zhex> flow "Ada"
....>     greet
....>     print
....>
Hello, Ada!
zhex> word greet(name: String) -> String
....>     "Hi, " + name + "!"
....>
redefined greet
zhex> greet("Grace")
Hi, Grace!
zhex> let name = read-line("Name: ") try
Name: Linus
zhex> greet(name)
Hi, Linus!
zhex> [Ctrl-D]
```

The lines after `zhex> ` and `....> ` are your input, as is `Linus` at the prompt; `[Ctrl-D]` is the keypress that exits with status 0. You can also call the preloaded REPL word `bye()` to exit. It appears in `words`, follows normal word-redefinition rules, and is not available in source files. The continuation spaces are editable: Tab inserts four spaces; Backspace removes one character, so press it four times to dedent one level. An empty or spaces-only continuation line submits the entry; Ctrl-C cancels the current entry. Ctrl-D on an empty or spaces-only continuation line in a pending entry discards it and exits. Redefining a word replaces it and prints `redefined greet`. A bare expression echoes its value, while `let` and `print` entries do not. Errors print a diagnostic and the session continues; `words` lists effective parameter, return, and Result signatures, with a documented word's `##` doc indented beneath its signature. A comment-only line never submits an entry; it joins the next line's entry, so a `##` doc block stays with the word it documents, and `words` typed after a comment line still runs. Auto-indentation applies only to interactive terminals. Piped entries keep the existing source format and need explicit four-space indentation; bracketed paste preserves its source indentation, and tabs and non-ASCII whitespace outside strings or comments remain source errors. Open `()`, `[]`, and `{}` delimiters continue across lines; blank and comment-only lines inside them do not submit. Closing a top-level expression submits it immediately; a pending block still needs a blank line. A program's `read-line` input is data and receives no source indentation or Tab expansion. See the [REPL guide](../site/src/content/docs/repl.md) for entry rules and tracing.

## Read the greeting

The [greeting source](../examples/hello.zhex) is:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = read-line "Your name: " try

flow name
    greet
    print
```

A **word** is a function with named, typed parameters. `greet` accepts a `String` and returns a `String`. Its final expression supplies the result. Blocks use four spaces, with no braces or semicolons. A contiguous block of `##` comment lines directly above a `word` declaration becomes that word's doc, shown by `words` beneath the signature. A doc line of the form `## > expression` is a doctest: `zhex doctests <file.zhex>` evaluates the expression in the word's own module scope and compares the doc lines that follow — until a blank doc line, the next `## >` line, or the doc's end — with the value as the REPL echoes it. A blank doc line ends the expected output, so prose after it is prose again; prose directly after an example reads as output and mismatches. `zhex run` never executes docs.

A `*.test.zhex` file is a plain program named after what it tests: `zhex test` (or `zhex test <path>` to narrow) runs every one under the working folder as its own process — it finishes, it passed — plus every other `.zhex` file's doctests, printing one PASS/FAIL line per file with the captured failure line and a summary such as `3 files: 2 passed, 1 failed`. The one-word `test` module ships the assertion: place it beside the module under test and call `test.check(condition, label)` — a false condition fails the run with its label. The [math module tests](../examples/modules/math.test.zhex) show the pattern.

`let` names an immutable value. Its type is inferred: the successful result of `read-line` is a string, so `name` is a `String` without another annotation.

`flow` sends each result to the next operation. Here `name` goes into `greet`, and the greeting goes into `print`. The checker verifies each connection before execution. You can call the word directly with parentheses, such as `greet(name)`. See [words and flows](../site/src/content/docs/words-and-flows.md) for additional arguments and more composition patterns.

## Read numbers and choose a fallback

Read a line, then parse it with `text.to-int`, the inverse of `int.to-string`. This program uses `try` for input failure and catches malformed numeric text:

```zhex
let line = read-line("Number: ") try
let number = text.to-int(line) catch problem
    print("Invalid integer: " + error.code(problem))
    0
print(number * number)
```

`read-line` returns a Result; `try` unwraps a successful line or propagates an input error. Here, invalid numeric text or an out-of-range integer is caught, its stable code is inspected, and the program substitutes `0`. The handler runs only when parsing fails.

`read-line` preserves spaces and accepts an empty line. `text.to-int` trims surrounding whitespace and requires a signed 64-bit decimal integer. Prompts are written and flushed before reading. `print` accepts an integer, boolean, or string and appends a newline.

Word parameter annotations can be omitted when the body or calls in the same program determine one concrete type; unresolved parameters need annotations. A return annotation can also be omitted when the body determines one concrete success type. When an unhandled `try` can fail, the checker infers an effective `Result` signature, and each caller uses `try` or `catch`. The [Words and flows guide](../site/src/content/docs/words-and-flows.md) explains inference and composition; the [Results and error handling guide](../site/src/content/docs/results.md) explains propagation and recovery.

## Make decisions

The [FizzBuzz example](../examples/fizzbuzz.zhex) uses conditions to return one kind of value from several branches:

```zhex
word fizzbuzz(n: Int) -> String
    if n % 15 == 0
        "FizzBuzz"
    elif n % 3 == 0
        "Fizz"
    elif n % 5 == 0
        "Buzz"
    else
        int.to-string(n)
```

Conditions must be booleans. Branch result types must agree. Here every branch returns a `String`; `int.to-string` makes the final conversion explicit. There is no automatic conversion between strings and integers.

## Repeat work and update values

A range includes its start and excludes its end. This program prints `10`:

```zhex
var total = 0

for number in range(1, 5)
    total = total + number

print(total)
```

`var` explicitly permits reassignment. Its initial value determines its type, and assignments must preserve that type. Use `let` when a value should stay fixed. The iteration variable is immutable and local to the loop.

[Fibonacci](../examples/fibonacci.zhex) and [factorial](../examples/factorial.zhex) use this approach for iterative calculations. Their input bounds avoid overflowing an `Int`; arithmetic still checks overflow independently. [GCD](../examples/gcd.zhex) recurses and feeds a computed call into a flow, while the [times table](../examples/times-table.zhex) builds each row from integers and strings.

## Try the text example

Run the [palindrome program](../examples/palindrome.zhex):

```sh
zhex run examples/palindrome.zhex
```

Try `racecar`, `hello`, or `été`. The program compares the input with `text.reverse(input)`. Reversal operates on Unicode code points. The check is exact: case, spaces, punctuation, and normalization are not changed.

## Read a file

`for line in file.stream(path)` streams a file's chomped lines and always closes the stream:

```zhex
for line in file.stream("examples/text-stats-sample.txt")
    print(text.upper(line))
```

Run it from the repository root. The [text-stats example](../examples/text-stats.zhex) demonstrates whole-file counts and a streamed uppercase echo:

```sh
zhex run examples/text-stats.zhex examples/text-stats-sample.txt
```

With one argument it reads the whole file and prints `3 lines  11 words  64 chars`; an additional argument streams the file uppercased line by line. The path comes from `console.arg(0) try`, and `console.args-count()` reports the number of arguments after the source path.

## Understand an error

A type error such as `print("hello" + 1)` is rejected before the program reads input or writes output. Runtime failures can happen after a prompt or earlier output. Diagnostics include the source filename, line, and column. The [diagnostics guide](../site/src/content/docs/diagnostics.md) shows annotated errors and machine-readable reports.

The built executable returns status `0` after a successful run, `1` for file or language errors, and `2` for invalid command usage. Identifiers may contain hyphens, as in `is-palindrome`. Put spaces around subtraction between names: `a - b` subtracts; `a-b` names a value or word.

## Compile to WebAssembly

`zhex compile examples/hello.zhex` checks the program exactly like `zhex run`, then assembles a standalone WebAssembly module, writing `examples/hello.wasm` beside the source (`-o` picks another path). The default `--target=browser` builds a module for a web page or JavaScript host; `--target=wasi` builds a standalone module for a runner such as wasmtime:

```sh
zhex compile examples/hello.zhex --target=wasi
printf 'Ada\n' | wasmtime run --dir .::/ examples/hello.wasm
```

Compiling needs the free wasm-tools assembler — install it with `cargo install wasm-tools`, `pacman -S wasm-tools`, or `brew install wasm-tools`; a release binary from the BytecodeAlliance releases page also works. Without it, `zhex compile` stops with the coded `missing-tool` diagnostic and those three install hints, and no artifact is written. Running a compiled module needs nothing extra in a browser; standalone runs need a runner such as wasmtime. The [compiling guide](../site/src/content/docs/compiling.md) covers targets, the wasm-tools requirement, and which constructs compile today.

## Common problems

- `zhex: command not found` — this terminal's `PATH` does not include the binary. Run the `export PATH` (or `$env:Path`) line from [Run your first program](#run-your-first-program) again, or run `just install` once for a permanent install.
- `go` is not found or older than 1.23 — check with `go version`, then follow the per-OS install steps in [Install the tools](#install-the-tools).
- `go build` fails — the two usual causes are an old Go (`go version` must report 1.23 or newer) and no network on the first build, which downloads the project's small Go modules once. Update Go, or build once from a connected terminal.
- `just test` or `just site` reports `missing-tool` — the website build needs the free wasm-tools assembler; install it with the one-line command the diagnostic prints (`cargo install wasm-tools`, `pacman -S wasm-tools`, or `brew install wasm-tools`); a release binary from the [wasm-tools releases page](https://github.com/bytecodealliance/wasm-tools/releases) also works.

When a run prints a coded diagnostic, its meaning is cataloged on the [Errors page](https://desireco.github.io/zhex/errors.html).

## Explore and validate

Start with the [examples and expected results](syntax-examples.md), then consult the [language contract](initial-language.md) for exact rules. The [test catalog](tests.md) maps behavior to its executable checks.
