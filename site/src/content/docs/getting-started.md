---
title: Getting started
description: Build Zhex, create a small program, run it, and try the REPL.
section: Start
order: 1
---

Build the interpreter, create `hello.zhex`, and run it from your terminal. You need Git, Go 1.23 or newer, and [`just`](https://just.systems/).

## Build and run

Clone the repository, build the interpreter once, and add it to this terminal's `PATH`:

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
just build
export PATH="$PWD/bin:$PATH"
```

The `export` command applies to the current shell. In a new terminal, run it again after changing to the repository.

## Create and run a program

Create `hello.zhex` in the repository root with this line:

```zhex
print("Hello, Zhex!")
```

Run it with `zhex run`:

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

let name = read-line("Your name: ") try

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

```text
Your name: Hello, Ada!
```

## Try the REPL

Start an interactive session. Calls use parentheses, so `print(1 + 1)` prints `2`:

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

Auto-indentation applies to interactive terminal input. Piped REPL input keeps the existing source format, so include the leading four spaces yourself. Pasted code keeps its source indentation; tabs and non-ASCII whitespace outside strings or comments are still rejected by the language. A program's `read-line` prompt reads input as data, without source auto-indentation or Tab expansion. The language grammar is unchanged.

Words and values stay available in later entries.

## Where to go next

- [Syntax](syntax.html): names, expressions, and indentation.
- [Values](values.html): bindings, types, and operators.
- [Words and flows](words-and-flows.html): composition, parameters, and results.
- [Control flow](control-flow.html): conditions and loops.
- [Examples](../examples.html): complete programs with source, input, and output.
