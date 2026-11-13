---
title: Words and flows
description: Define typed words and compose them with direct calls or checked one-argument flows.
section: Language
order: 30
---

A word is a named operation with typed parameters and a result type. The type checker checks every word body, including words the program never calls.

## Define a word

Write the parameter names and types, the result type, then an indented body. The final statement supplies the result:

```zhex
word square(n: Int) -> Int
    let result = n * n
    result

print(square(7))
```

Each parameter and the result needs a type. Bindings, assignments, and loops have type `Unit`; a word's declared result must match its final statement's type. An `if` or a `flow` can produce a value.

Definitions are top-level. Words may call later definitions in the same file, and a word may call itself recursively. Recursive execution has a depth limit of 256. Words do not capture top-level local values, so values a word needs must be passed as parameters.

## Call a word

Use parentheses for a direct call. Arguments are evaluated left to right and checked against the parameter types:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

let greeting = greet("Ada")
print(greeting)
```

## Compose a flow

A `flow` evaluates its seed expression, then passes the current value to each single-argument word on the next indented lines. Each step's input type must match the preceding value; the flow result is the final step's result.

```zhex
word square(n: Int) -> Int
    n * n

word double(n: Int) -> Int
    n + n

flow 4
    square
    double
    print
```

This prints `32`. A flow can start from any expression, including a call with multiple arguments:

```zhex
word gcd(a: Int, b: Int) -> Int
    if b == 0
        a
    else
        gcd(b, a % b)

flow gcd(48, 18)
    print
```

The seed call is evaluated first and its result goes to `print`. A flow step is a word name, not a call with arguments. Use a direct call when an operation needs multiple explicit arguments.

See [modules](modules.html) for qualified word calls and [control flow](control-flow.html) for using flows inside loops.
