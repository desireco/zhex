# Getting started with Zhex

Zhex runs `.zhex` source files through a small Go interpreter. This guide introduces the supported syntax using console programs you can run and change.

## Run your first program

You need Go 1.23 or newer and access to the repository:

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
go run ./cmd/zhex run examples/hello.zhex
```

A terminal session looks like this:

```text
Your name: Ada
Hello, Ada!
```

`Ada` is the input you type. The prompt and greeting come from the program. You can supply input through a pipe too:

```sh
printf 'Ada\n' | go run ./cmd/zhex run examples/hello.zhex
```

With redirected input, the program writes `Your name: Hello, Ada!` followed by a newline; terminal input echo is not part of its output.

To build the interpreter once:

```sh
go build -o bin/zhex ./cmd/zhex
./bin/zhex run examples/hello.zhex
```

## Try it interactively

The interpreter also has a REPL that checks and runs each entry as you type it:

```sh
go run ./cmd/zhex repl
```

Words and values persist across entries. A word header starts a multi-line entry, marked by the `....> ` continuation prompt, and a blank line submits it:

```text
Zhex REPL — end multi-line entries with a blank line; Ctrl-D exits.
zhex> word greet(name: String) -> String
....>     "Hello, " + name + "!"
....>
zhex> greet("Ada")
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

The lines after the `zhex> ` and `....> ` prompts are your input, as is `Linus` at the `Name:` prompt; `[Ctrl-D]` is the keypress that ends the session with exit status 0. Redefining a word replaces it and prints `redefined greet`. An entry that is a bare expression echoes its value, which is how `greet("Ada")` shows its greeting; `let` and `print` entries produce no echo. Statements never span lines — only a missing indented body continues an entry, and a blank line ends it. Errors print their diagnostic and the session survives, `words` lists the signatures defined so far, and `--trace` streams the same step events as `zhex run --trace`.

## Read the greeting

The [greeting source](../examples/hello.zhex) is:

```text
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = read-line("Your name: ") try

flow name
    greet
    print
```

A **word** is a function. `greet` accepts a `String` and returns a `String`. Its final expression supplies the result. Blocks use four spaces, with no braces or semicolons.

`let` names an immutable value. Its type is inferred: the successful result of `read-line` is a string, so `name` is a `String` without another annotation.

`flow` expresses a sequence of one-argument operations. Here the name goes into `greet`, and the resulting greeting goes into `print`. The checker verifies each connection before execution. You can also call words directly with parentheses, such as `greet(name)`.

## Read numbers and choose a fallback

Read a line, then parse it with `text.to-int`, the inverse of `int.to-string`. This program uses `try` for input failure and handles malformed numeric text with `catch`:

```zhex
let line = read-line("Number: ") try
let number = text.to-int(line) catch problem
    print("Invalid integer: " + error.code(problem))
    0
print(number * number)
```

`read-line` returns a Result; `try` unwraps a successful line or propagates an input error to the caller. Here, invalid numeric text or an out-of-range integer is caught, its stable code is inspected, and the program substitutes `0`. The handler is lazy: it runs only when parsing fails.

`read-line` preserves spaces and accepts an empty line. `text.to-int` trims surrounding whitespace and requires a signed 64-bit decimal integer. Prompts are written and flushed before reading. `print` accepts an integer, boolean, or string and appends a newline.

If one word calls another fallible word, declare a compatible `Result` return and add `try` at the call site. The [Results and error handling guide](../site/src/content/docs/results.md) shows multi-level propagation, structured errors, and the optional reporter.

## Make decisions

The [FizzBuzz example](../examples/fizzbuzz.zhex) uses conditions to return one kind of value from several branches:

```text
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

```text
var total = 0

for number in range(1, 5)
    total = total + number

print(total)
```

`var` explicitly permits reassignment. The initial value determines its type, and assignments must preserve that type. Use `let` when the value should stay fixed. The iteration variable is immutable and local to the loop.

[Fibonacci](../examples/fibonacci.zhex) and [factorial](../examples/factorial.zhex) use this approach for iterative calculations. Their input bounds avoid overflowing an `Int`; arithmetic still checks overflow independently. [GCD](../examples/gcd.zhex) recurses — a word may call itself — and feeds a computed call into a flow, while the [times table](../examples/times-table.zhex) builds each row of a bounded loop from integers and strings.

## Try the text example

```sh
go run ./cmd/zhex run examples/palindrome.zhex
```

Try `racecar`, `hello`, or `été`. The [palindrome source](../examples/palindrome.zhex) compares the input with `text.reverse(input)`. Reversal operates on Unicode code points. The check is exact: case, spaces, punctuation, and normalization are not changed.

## Read a file

`for line in file.lines(path)` streams a file's chomped lines and always closes the file:

```text
for line in file.lines("examples/text-stats-sample.txt")
    print(text.upper(line))
```

Run it and the three sample lines print uppercased. The [text-stats source](../examples/text-stats.zhex) shows both halves of file reading:

```sh
go run ./cmd/zhex run examples/text-stats.zhex examples/text-stats-sample.txt
```

With one argument it reads the whole file at once and prints `3 lines  11 words  64 chars`; any second argument streams the file uppercased line by line, as in the loop above. The path comes from `console.arg(0) try`, and `console.args-count()` says how many arguments followed `zhex run <file>`.

## Understand an error

A type error such as `print("hello" + 1)` is rejected before any program input or output occurs. Runtime failures can happen after a prompt or earlier output. Diagnostics include the source filename, line, and column.

The built executable returns exit status `0` for success, `1` for file or language errors, and `2` for invalid command usage. If you invoke it through `go run`, Go may wrap the program's failure in its own diagnostic.

Identifiers may contain hyphens, as in `is-palindrome`. Put spaces around subtraction between names: `a - b` subtracts; `a-b` names a value or word.

## Explore and validate

Start with the [ten examples and expected results](syntax-examples.md), then consult the [language contract](initial-language.md) for exact rules. Run the executable language tests with:

```sh
go test ./...
```

The [test catalog](tests.md) explains which tests cover each feature. Collections, rich domain types, resource scopes, and native compilation are not part of the current interpreter.
