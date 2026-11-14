---
title: Words and flows
description: Define words with typed parameters and inferred or explicit return types, then compose them with flows.
section: Language
order: 30
---

A word is a named operation with typed parameters and a success type. The type checker checks every word body, including words the program never calls.

## Define a word

Write parameter names and types, then an indented body. A return annotation is optional; if omitted, the checker infers the word's success type from its body:

```zhex
word square(n: Int)
    n * n

print(square(7))
```

`square` has inferred success type `Int`; if the body does not determine one concrete type, add a return annotation. For example, a recursive cycle without a concrete type anchor or an empty list result needs an explicit type such as `-> Int` or `-> List<Int>`.

Parameter annotations can also be omitted when the word body or calls in the same program determine one concrete type:

```zhex
word character-count(value)
    text.length(value)

word identity(value)
    value

print(character-count("hello"))
print(identity(7))
```

This prints `5` and `7`. `text.length` constrains `character-count`'s parameter to `String`; the call to `identity(7)` supplies the constraint for `identity`. Explicit and inferred parameter types can be mixed, as in `word count-down(n, prefix: String)`. If a parameter remains unconstrained, add its type annotation; conflicting calls are type errors, and a word parameter cannot have a `Result` type. Bindings, assignments, and loops have type `Unit`; an explicit return type must match the final statement's type. An `if`, `match`, or `flow` can produce a value. Escaping failures can give a user word an inferred effective `Result` signature; see [results and error handling](results.html).

`if`, `match`, and `flow` blocks can also supply the entire right-hand side of a `let`, `var`, or assignment. Their final value becomes the binding or assignment value:

```zhex
let normalized = flow "  ADA  "
    text.trim
    text.lower

var selection = if true
    "first"
else
    "second"

selection = match Option.Some("updated")
    case Option.Some(value)
        value
    case Option.None
        "missing"

print(normalized)
print(selection)
```

This prints `ada` and `updated`. These are whole right-hand-side blocks; they do not become general nested expressions or arguments to calls.

Definitions are top-level. Words may call later definitions in the same file, and a word may call itself recursively. Recursive execution has a depth limit of 256. Words do not capture top-level local values, so values a word needs must be passed as parameters.

## Call a word

Use parentheses for a direct call. Arguments are evaluated left to right and checked against the parameter types:

```zhex
word greet(name: String) -> String
    "Hello, " + name + "!"

print(greet("Ada"))
```

Simple calls may also use a comma-separated short form, such as `greet "Ada"`; this is useful for a complete statement, binding right-hand side, final word value, or flow step. Keep parentheses for nested calls and ambiguous arguments. See [calls and defaults](calls.html) for the exact boundaries and runnable examples.

## Default parameters

Parameters with defaults must come after required parameters. Calls stay positional and may omit only the trailing defaulted arguments:

```zhex
word default-name()
    print("default ran")
    "Ada"

word greet(prefix: String, name = default-name())
    prefix + " " + name

print(greet("Hello", "Grace"))
print(greet("Hello"))
```

This prints `Hello Grace`, then `default ran`, then `Hello Ada`. The default expression runs only when its argument is omitted. Supplied arguments evaluate once from left to right first; omitted defaults then evaluate in parameter order. A default can infer its parameter's type, as `name` does here. You can also annotate it explicitly (`name: String = default-name()`). Word calls do not support named arguments; named constructor arguments apply only to record constructors, as described in [records](records.html).

Defaults are evaluated in the word's declaring module. They cannot read parameters, globals, or caller bindings, and they cannot let a `Result` failure escape; a default may call a helper that handles its own failure. Each invocation evaluates omitted defaults afresh, so mutable defaults are not shared between calls:

```zhex
word append-one(values: List<Int> = [])
    list.append(values, 1)
    list.length(values)

print(append-one())
print(append-one())
```

This prints `1` twice because each call gets a new empty list. The [word defaults example](../examples.html#word-defaults) also shows lazy evaluation and inferred parameter types.

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
word parse(raw: String)
    flow raw
        text.trim
        text.to-int try

flow "42"
    parse try
    print
```

The body infers success type `Int`; because parsing can fail, `parse` is displayed to callers as `Result<Int, ParseError>`. The flow trims the input, parses it, then prints the successful integer. A bare `parse` step is rejected because it would discard a possible failure. See [results and error handling](results.html) for inferred return types and effects, explicit `Result` signatures, propagation, and recovery.

See [modules](modules.html) for qualified word calls and [control flow](control-flow.html) for using flows inside loops.
