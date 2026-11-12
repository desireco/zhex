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

The command currently runs files. An interactive language REPL is planned.

## Read the greeting

The [greeting source](../examples/hello.zhex) is:

```text
word greet(name: String) -> String
    "Hello, " + name + "!"

let name = console.read-line("Your name: ") try

flow name
    greet
    print
```

A **word** is a function. `greet` accepts a `String` and returns a `String`. Its final expression supplies the result. Blocks use four spaces, with no braces or semicolons.

`let` names an immutable value. Its type is inferred: the successful result of `console.read-line` is a string, so `name` is a `String` without another annotation.

`flow` expresses a sequence of one-argument operations. Here the name goes into `greet`, and the resulting greeting goes into `print`. The checker verifies each connection before execution. You can also call words directly with parentheses, such as `greet(name)`.

## Read numbers and handle failure

Use `console.read-int` for a signed 64-bit integer:

```text
let number = console.read-int("Number: ") try
print(number * number)
```

Both input operations return a result that can succeed or fail. Postfix `try` unwraps success and propagates failure to the runner. Invalid numeric text, an out-of-range integer, or end-of-input produces a diagnostic and a nonzero exit status. Input failure never silently becomes zero or an empty string.

`console.read-line` preserves spaces and accepts an empty line. `console.read-int` trims surrounding whitespace and requires a decimal integer. Prompts are written and flushed before reading. `print` accepts an integer, boolean, or string and appends a newline.

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

[Fibonacci](../examples/fibonacci.zhex) and [factorial](../examples/factorial.zhex) use this approach for iterative calculations. Their input bounds avoid overflowing an `Int`; arithmetic still checks overflow independently.

## Try the text example

```sh
go run ./cmd/zhex run examples/palindrome.zhex
```

Try `racecar`, `hello`, or `été`. The [palindrome source](../examples/palindrome.zhex) compares the input with `text.reverse(input)`. Reversal operates on Unicode code points. The check is exact: case, spaces, punctuation, and normalization are not changed.

## Understand an error

A type error such as `print("hello" + 1)` is rejected before any program input or output occurs. Runtime failures can happen after a prompt or earlier output. Diagnostics include the source filename, line, and column.

The built executable returns exit status `0` for success, `1` for file or language errors, and `2` for invalid command usage. If you invoke it through `go run`, Go may wrap the program's failure in its own diagnostic.

Identifiers may contain hyphens, as in `is-palindrome`. Put spaces around subtraction between names: `a - b` subtracts; `a-b` names a value or word.

## Explore and validate

Start with the [six examples and expected results](syntax-examples.md), then consult the [language contract](initial-language.md) for exact rules. Run the executable language tests with:

```sh
go test ./...
```

The [test catalog](tests.md) explains which tests cover each feature. Modules, collections, rich domain types, resource scopes, and native compilation are not part of the current interpreter.
