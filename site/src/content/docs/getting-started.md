---
title: Getting started
description: Build Zhex, create a small program, run it, and try the REPL.
section: Start
order: 1
---

Build the interpreter, create `hello.zhex`, and run it from your terminal. You need Git, Go 1.23 or newer, and [`just`](https://just.systems/).

## Build Zhex

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

Now replace the file with a word that reads a name and sends it through a flow:

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

Start an interactive session:

```sh
zhex repl
```

Define and call a word. The blank line submits its body:

```text
zhex> word greet(name: String) -> String
....>     "Hello, " + name + "!"
....>
zhex> greet("Ada")
Hello, Ada!
```

Words and values stay available in later entries. Press Ctrl-D to exit.

## Continue learning

- [Syntax](syntax.html): names, expressions, and indentation.
- [Values](values.html): bindings, types, and operators.
- [Words and flows](words-and-flows.html): composition, parameters, and results.
- [Control flow](control-flow.html): conditions and loops.
- [Examples](../examples.html): complete programs with source, input, and output.
