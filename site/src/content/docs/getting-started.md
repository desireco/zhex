---
title: Getting started
description: Build Zhex, create a small program, run it, and try the REPL.
section: Start
order: 1
---

Build the interpreter, create `hello.zhex`, and run it from your terminal. You need Git and Go 1.23 or newer — both come from your system's package manager.

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

## Build and run

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

## Create and run a program

Create `hello.zhex` in the repository root with this line, using any text editor — Notepad works:

```zhex
print("Hello, Zhex!")
```

Run it with `zhex run` — the command is the same on every platform:

```sh
zhex run hello.zhex
```

```text
Hello, Zhex!
```

## Read the program

Replace the file with a word that reads a name and sends it through a flow:

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

`greet` accepts and returns a `String`. The word's final expression is its result. `read-line` can fail, so `try` handles its `Result`; `flow` passes the name to `greet`, then passes the greeting to `print`. Zhex checks word names, parameter types, and each step of the flow before execution.

For a file you already have, run `zhex run examples/hello.zhex`. With piped input, the terminal does not echo the typed text:

```sh
printf 'Ada\n' | zhex run examples/hello.zhex
```

```powershell
'Ada' | zhex run examples\hello.zhex
```

```text
Your name: Hello, Ada!
```

## Try the REPL

Start an interactive session. Parenthesized calls remain available; simple calls may also use comma-separated arguments at a complete expression boundary, so `print 1 + 1` prints `2`:

```sh
zhex repl
```

Define a word and send a value through it. The terminal editor inserts four spaces after each block header, including nested ones. A blank line submits the entry; the flow body is indented four spaces:

```text
zhex> print(1 + 1)
2
zhex> word greet(name: String) -> String
....>     "Hello, " + name + "!"
....>
zhex> flow "Ada"
....>     greet
....>     print
....>
Hello, Ada!
```

The spaces shown after `....>` are editable. Tab inserts four spaces; Backspace removes one character; press it four times to dedent one level. An empty or spaces-only continuation line submits the entry. While editing, Ctrl-C cancels the current entry. Ctrl-D on an empty top-level line exits; in a pending entry, Ctrl-D on an empty or spaces-only continuation line discards it and exits.

An open `()`, `[]`, or `{}` also continues an entry. Blank and comment-only lines inside it do not submit. A comment-only line outside a delimiter never completes an entry either: it joins the next line's entry, so a `##` doc block stays with the word it documents. Closing a top-level expression submits it; if it is inside a pending block, a blank line still submits the block. This works in source files and the REPL.

Auto-indentation applies to interactive terminal input. Piped REPL input keeps the existing source format, so include the leading four spaces yourself. Pasted code keeps its source indentation; tabs and non-ASCII whitespace outside strings or comments are still rejected by the language. A program's `read-line` prompt reads input as data, without source auto-indentation or Tab expansion.

Words and values stay available in later entries. Ctrl-D exits with status 0; you can also call `bye()` to exit.

## Where to go next

- [Syntax](syntax.html): names, expressions, and indentation.
- [Values](values.html): bindings, types, and operators.
- [Words and flows](words-and-flows.html): composition, parameters, and results.
- [Control flow](control-flow.html): conditions and loops.
- [Examples](../examples.html): complete programs with source, input, and output.
