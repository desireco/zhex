# Getting started

Build the interpreter, create a small Zhex program, and run it. You need Git, Go 1.23 or newer, and [`just`](https://just.systems/).

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

Run it:

```sh
zhex run hello.zhex
```

```text
Hello, Zhex!
```

Replace the file contents with a word that reads a name and sends it through a flow:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = read-line("Your name: ") try

flow name
    greet
    print
```

Run `zhex run hello.zhex` again and enter `Ada` at the prompt:

```text
Your name: Ada
Hello, Ada!
```

`greet` accepts and returns a `String`; the final expression in a word is its result. `read-line` can fail, so `try` handles its `Result`. The `flow` sends the name to `greet`, then sends the greeting to `print`. The checker verifies word names, parameter types, and flow connections before execution.

To run an included example, use `zhex run examples/hello.zhex`. When input comes from a pipe, the terminal does not echo the typed text:

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

- [Syntax examples](syntax-examples.md): complete programs with expected output and edge cases.
- [Language contract](initial-language.md): exact syntax and semantics.
- [Test catalog](tests.md): where each behavior is checked.
