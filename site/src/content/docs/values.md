---
title: Values and operators
description: Reference Zhex value types, bindings, lexical scope, operators, and checked integer arithmetic.
section: Language
order: 20
---

Every expression has a checked type. Local binding types are inferred from their initial value unless an explicit `let name: Type = value` or `var name: Type = value` annotation is supplied. Word parameter and result types are written explicitly. Zhex has no implicit conversions.

## Types

| Type | Values and use |
| --- | --- |
| `Int` | Signed 64-bit integers. |
| `Bool` | `true` or `false`. |
| `String` | Valid UTF-8 text. |
| `Unit` | The result of statements such as assignment, loops, and `print`. |
| Temporal values | `Date`, `Time`, `Duration`, `Instant`, `TimeZone`, and `ZonedDateTime` represent distinct calendar and elapsed-time concepts. See [dates and time zones](temporal.html). |
| `Buffer` | A mutable sequence of string lines. See [buffers](buffers.html). |
| `File` | An open file handle. See [files](files.html). |
| Result values | Input and parsing operations return `Result<T, E>`; selected collection operations also offer checked forms. User words declare a success type; an escaping `try` makes the checker infer the effective Result signature. Explicit `Result<T, E>` signatures remain supported. Results cannot be used as ordinary values and must be handled immediately with `try` or `catch`. User-defined error types are not supported. See [results](results.html). |
| Records | Nominal named types with typed, immutable fields. They cannot be compared or printed directly. See [records](records.html). |
| Choices | Nominal named types with a fixed set of alternatives and optional typed payloads; a choice can declare one type parameter. Use exhaustive `match` to select a case; choices cannot be compared or printed directly. See [choices and matching](choices.html). |
| `List<T>` / `Array<T>` | Homogeneous, shared mutable collections. They cannot be compared or printed directly. See [lists and arrays](collections.html). |
| `Map<K, V>` | Shared mutable key/value collection with insertion-ordered iteration. Keys are `String`, `Int`, or `Bool`; lookup returns built-in `Option<V>`. Maps cannot be printed or compared directly. See [maps](maps.html). |

`Buffer` and `File` are resource values. They are not copied when a name is bound, cannot be compared with `==` or `!=`, and are not accepted by `print`.

The [JSON codec](json.html) supports typed records, lists, string-keyed maps, and `Option` values in addition to `Int`, `Bool`, and `String`.

## Bindings and scope

`let` creates an immutable binding. `var` creates a binding that can later be assigned a value of the same type:

```zhex
let greeting = "Hello"
var count = 0
count = count + 1
print(greeting + " " + int.to-string(count))
```

Bindings live in lexical block scope. A nested block may shadow an outer name; two bindings with the same name in one scope are an error. Word parameters are immutable. A word does not capture top-level local bindings; pass values through parameters instead.

An `if`, `match`, or `flow` block may supply the entire right-hand side of a `let`, `var`, or assignment. See [words and flows](words-and-flows.html) for a complete example.

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
