---
title: Getting started
description: Build Zhex, run your first program, and learn the basic shape of a word and a flow.
section: Start
order: 1
---

Zhex runs `.zhex` source files with a checked interpreter. This guide gets the interpreter built, runs a program, and introduces the syntax you will use in the rest of the [language guide](syntax.html).

## Build and run

Clone the repository and build the interpreter. The build needs Go 1.23 or newer and [`just`](https://just.systems/).

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
just build
export PATH="$PWD/bin:$PATH"
zhex run examples/hello.zhex
```

The program asks for a name. Type `Ada` and press Enter:

```text
Your name: Ada
Hello, Ada!
```

`zhex run` checks the complete program before it reads input or writes output. You can pipe input into it too:

```sh
printf 'Ada\n' | zhex run examples/hello.zhex
```

When input is piped, the terminal does not echo `Ada`, so the prompt and greeting appear on the same output line.

## Read the program

The greeting example defines a word, reads a line, then passes the value through a flow:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = read-line("Your name: ") try

flow name
    greet
    print
```

A **word** is a function with named, typed parameters. Its final statement is the result. `let` gives a value a name; here the result of `read-line` is unwrapped with `try`. The `flow` sends `name` to `greet`, then sends the greeting to `print`. The checker verifies each step's type before the program runs.

Blocks are indented with four spaces. There are no braces or semicolons. A blank line can separate top-level pieces for readability.

You can call a word directly too: `greet(name)`. See [words and flows](words-and-flows.html) for signatures, recursion, and composition.

## Try the REPL

The interactive interpreter checks and runs entries as you type them:

```sh
zhex repl
```

Definitions and values persist across entries. A blank line submits a multi-line word definition. The [REPL guide](repl.html) covers entry boundaries, errors, and `words`.

## Where to go next

- [Syntax](syntax.html): indentation, names, comments, and expressions.
- [Values and operators](values.html): types, bindings, scope, and arithmetic.
- [Control flow](control-flow.html): conditions and loops.
- [Examples](../examples.html): complete programs with real input and output.
- [Language contract](https://github.com/desireco/zhex/blob/main/docs/initial-language.md): detailed current semantics.
