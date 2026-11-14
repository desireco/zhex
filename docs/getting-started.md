# Getting started

Build the interpreter, create a small Zhex program, and run it. You need Git, Go 1.23 or newer, and [`just`](https://just.systems/). For the shorter public tutorial, see [Getting started on the website](https://desireco.github.io/zhex/docs/getting-started.html).

## Run your first program

Clone the repository, build the interpreter once, and add it to this terminal's `PATH`:

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
just build
export PATH="$PWD/bin:$PATH"
```

The `export` command applies to the current shell. In a new terminal, run it again after changing to the repository.

Create `hello.zhex` in the repository root with this line:

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

Replace the file contents with a word that reads a name and sends it through a flow:

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

`greet` accepts and returns a `String`; its final expression is the result. `read-line` can fail, so `try` handles its `Result`. The flow sends the name to `greet`, then sends the greeting to `print`. The checker verifies word names, parameter types, and flow connections before execution.

For an included program, run `zhex run examples/hello.zhex`. Piped input is not echoed by the terminal:

```sh
printf 'Ada\n' | zhex run examples/hello.zhex
```

```text
Your name: Hello, Ada!
```

## Try it interactively

Start a REPL session:

```sh
zhex repl
```

Words and values persist across entries. Calls use parentheses: `print(1 + 1)` prints `2`. A word header opens a multi-line entry, shown with the `....> ` continuation prompt. In an interactive terminal, the editor pre-fills four spaces and carries indentation into nested blocks:

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

The lines after `zhex> ` and `....> ` are your input, as is `Linus` at the prompt; `[Ctrl-D]` is the keypress that exits with status 0. You can also call the preloaded REPL word `bye()` to exit. It appears in `words`, follows normal word-redefinition rules, and is not available in source files. The continuation spaces are editable: Tab inserts four spaces; Backspace removes one character, so press it four times to dedent one level. An empty or spaces-only continuation line submits the entry; Ctrl-C cancels the current entry. Ctrl-D on an empty or spaces-only continuation line in a pending entry discards it and exits. Redefining a word replaces it and prints `redefined greet`. A bare expression echoes its value, while `let` and `print` entries do not. Errors print a diagnostic and the session continues; `words` lists the effective signatures defined so far, including inferred Result effects. Auto-indentation applies only to interactive terminals. Piped entries keep the existing source format and need explicit four-space indentation; bracketed paste preserves its source indentation, and tabs and non-ASCII whitespace outside strings or comments remain source errors. Open `()`, `[]`, and `{}` delimiters continue across lines; blank and comment-only lines inside them do not submit. Closing a top-level expression submits it immediately; a pending block still needs a blank line. A program's `read-line` input is data and receives no source indentation or Tab expansion. See the [REPL guide](../site/src/content/docs/repl.md) for entry rules and tracing.

## Read the greeting

The [greeting source](../examples/hello.zhex) is:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = read-line("Your name: ") try

flow name
    greet
    print
```

A **word** is a function with named, typed parameters. `greet` accepts a `String` and returns a `String`. Its final expression supplies the result. Blocks use four spaces, with no braces or semicolons.

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

Word parameters remain explicitly typed, but a return annotation can be omitted when the body determines one concrete success type. When an unhandled `try` can fail, the checker infers an effective `Result` signature, and each caller uses `try` or `catch`. The [Results and error handling guide](../site/src/content/docs/results.md) explains propagation, structured errors, and recovery.

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

`for line in file.lines(path)` streams a file's chomped lines and always closes the stream:

```zhex
for line in file.lines("examples/text-stats-sample.txt")
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

## Explore and validate

Start with the [examples and expected results](syntax-examples.md), then consult the [language contract](initial-language.md) for exact rules. The [test catalog](tests.md) maps behavior to its executable checks.
