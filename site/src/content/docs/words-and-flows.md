---
title: Words and flows
description: Define typed words and compose them with flows that carry values and explicit arguments.
section: Language
order: 30
---

A word is a named operation with typed parameters and a result type. The type checker checks every word body, including words the program never calls.

## Define a word

Write the parameter names and types, the result type, then an indented body. The final statement supplies the result:

```zhex
word square(n: Int) -> Int
    n * n

print(square(7))
```

Each parameter and success type needs a type. Bindings, assignments, and loops have type `Unit`; a word's declared success type must match its final statement's type. An `if` or a `flow` can produce a value. Escaping failures can give a user word an inferred effective `Result` signature; see [results and error handling](results.html).

Definitions are top-level. Words may call later definitions in the same file, and a word may call itself recursively. Recursive execution has a depth limit of 256. Words do not capture top-level local values, so values a word needs must be passed as parameters.

## Call a word

Use parentheses for a direct call. Arguments are evaluated left to right and checked against the parameter types:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

print(greet("Ada"))
```

## Compose a flow

A `flow` evaluates its seed expression, then passes the current value to each step on the next indented lines. A bare word step uses that value as its only argument. Each step's input type must match the preceding value; the flow result is the final step's result. A fallible step must be marked `try`, just like a direct call.

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

The seed call is evaluated first and its result goes to `print`. A unary flow step names a word; a call-shaped step supplies additional arguments after the current value, which is passed as the first argument:

```zhex
word add(a: Int, b: Int) -> Int
    a + b

flow 4
    add(3)
    print
```

This prints `7`. The current value supplies `a`, and `3` supplies `b`. Explicit arguments evaluate once from left to right after the flow value. Use the call form when the next operation needs more than the current value.

Collection operations can use the same form. For example, a map can be piped through `map.put("Nancy", 23)`; the map is the implicit first argument, and the key and value are the explicit arguments. See [maps](maps.html) for a complete example.

When each step transforms the previous value once, connect the steps directly:

```zhex
flow "  Ada  "
    text.trim
    text.upper
    print
```

This prints `ADA`. Using `square` from above, a flow may also start with a fallible expression:

```zhex
flow text.to-int("7") try
    square
    print
```

This prints `49`. Give a value a name when you reuse it or when the name clarifies the state; flows make a one-pass transformation visible from seed to output.

An ordinary result type such as `Int` names a word's success value. If a `try` inside the word can fail, the checker infers the escaping error family and exposes an effective `Result` signature. Mark the call with `try` at each caller:

```zhex
word parse(raw: String) -> Int
    flow raw
        text.trim
        text.to-int try

flow "42"
    parse try
    print
```

`parse` is displayed to callers as `Result<Int, ParseError>`. The flow trims the input, parses it, then prints the successful integer. A bare `parse` step is rejected because it would discard a possible failure. See [results and error handling](results.html) for inferred effects, explicit `Result` signatures, propagation, and recovery.

See [modules](modules.html) for qualified word calls and [control flow](control-flow.html) for using flows inside loops.
