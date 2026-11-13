---
title: Values and operators
description: Reference Zhex value types, bindings, lexical scope, operators, and checked integer arithmetic.
section: Language
order: 20
---

Every expression has a checked type. Local binding types are inferred from their initial value; word parameter and result types are written explicitly. Zhex has no implicit conversions.

## Types

| Type | Values and use |
| --- | --- |
| `Int` | Signed 64-bit integers. |
| `Bool` | `true` or `false`. |
| `String` | Valid UTF-8 text. |
| `Unit` | The result of statements such as assignment, loops, and `print`. |
| `Buffer` | A mutable sequence of string lines. See [buffers](buffers.html). |
| `File` | An open file handle. See [files](files.html). |
| Result values | Selected built-ins return either a success value or a failure. These built-in result families must be unwrapped with `try`; they cannot be used as ordinary values. `Result<T, E>` is descriptive notation here, not a type you can write in a signature or define yourself. See [results](results.html). |

`Buffer` and `File` are resource values. They are not copied when a name is bound, cannot be compared with `==` or `!=`, and are not accepted by `print`.

## Bindings and scope

`let` creates an immutable binding. `var` creates a binding that can later be assigned a value of the same type:

```zhex
let greeting = "Hello"
var count = 0
count = count + 1
print(greeting + " " + int.to-string(count))
```

Bindings live in lexical block scope. A nested block may shadow an outer name; two bindings with the same name in one scope are an error. Word parameters are immutable. A word does not capture top-level local bindings; pass values through parameters instead.

## Operators

`+` adds two integers or concatenates two strings. `-`, `*`, `/`, and `%` operate on integers. Ordering operators require integers. Equality compares matching `Int`, `Bool`, or `String` values. `and`, `or`, and `not` operate on booleans.

```zhex
let total = 7 + 5
let is-even = total % 2 == 0
if is-even and total > 0
    print("positive even total")
```

The full precedence table is on the [syntax page](syntax.html#expression-shape). Binary operators associate left; `and` and `or` short-circuit.

## Checked arithmetic

Integers are signed 64-bit values. Overflow, division by zero, and remainder by zero are runtime errors with source locations. Division truncates toward zero. Remainder has the dividend's sign. The minimum integer divided by `-1` overflows; its remainder modulo `-1` is zero.

Use `int.to-string` to format an integer for a string. There is no automatic conversion between numbers and text.

For word signatures and how values move between operations, see [words and flows](words-and-flows.html). For branch and loop scope, see [control flow](control-flow.html).
