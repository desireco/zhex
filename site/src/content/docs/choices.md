---
title: Choices and matching
description: Declare a named set of alternatives and handle each case with an exhaustive match.
section: Language
order: 27
---

A choice type defines a fixed set of named alternatives. Each alternative can carry typed values, and a `match` handles the alternatives that a program expects.

```zhex
type Status = choice
    Pending
    Running
    Failed(message: String)

word describe(status: Status) -> String
    match status
        case Status.Pending
            "pending"
        case Status.Running
            "running"
        case Status.Failed(message)
            "failed: " + message

print(describe(Status.Running))
```

`type Name = choice` is followed by indented alternatives. An alternative without fields is constructed without parentheses, as `Status.Pending`. A payload alternative declares typed fields, then is constructed positionally in declaration order, as `Status.Failed("network unavailable")`. Choice values have the declared nominal type; alternatives from different types are distinct. The checker infers the choice type from a qualified constructor or checks it against an annotation or word signature.

## One-parameter choices

A choice can declare one type parameter and use it in alternative payloads:

```zhex
type Maybe<T> = choice
    None
    Some(value: T)

let answer = Maybe.Some(7)
let empty: Maybe<Int> = Maybe.None

word maybe-value(m: Maybe<Int>) -> Int
    match m
        case Maybe.Some(value)
            value
        case Maybe.None
            0

print(maybe-value(answer))
print(maybe-value(empty))
```

The payload of `Maybe.Some(7)` determines `Maybe<Int>`. A nullary alternative carries no value to infer the parameter from, so `Maybe.None` needs an expected `Maybe<T>` type from an annotation, parameter, return type, or surrounding expression. Type arguments can nest, such as `Maybe<List<Int>>`. The built-in `Option<T>` is reserved for map lookups; choose another name for a user choice. Generic words and generic records are not supported.

## Match every case

Use `match value` with an indented list of `case` arms. A case names an alternative and may bind its payload fields:

```zhex
match Status.Failed("network unavailable")
    case Status.Failed(message)
        print("Could not start: " + message)
    case Status.Pending
        print("Waiting to start")
    case Status.Running
        print("Already running")
```

The cases must cover every alternative exactly once unless the final arm is `case _`, which handles all remaining alternatives. Use `_` inside a payload pattern to ignore that field, as in `case Status.Failed(_)`. A wildcard is allowed at most once and must be last. Missing cases, repeated alternatives, a default after every alternative is already covered, and cases after a wildcard are checker errors.

Using the `Status` declaration above, a short match can handle one named alternative and use `_` for the rest:

```zhex
word short-label(status: Status) -> String
    match status
        case Status.Failed(_)
            "failed"
        case _
            "not failed"

print(short-label(Status.Failed("network unavailable")))
```

`match` can be a statement, the final statement of a word, or the entire right-hand expression of a `let` or `var` binding. It evaluates the matched value once. Payload names are immutable bindings scoped to their case body. Value-producing arms must have compatible result types; a case body can end in a `flow`. A branch that does not return can participate in a value-producing match; for example, `console.exit(1)` or `console.fail(message)` can end one branch while the other branches return a value. See [console](console.html) for those operations.

Choice payload fields cannot contain `Result` values, including through a type argument such as `Maybe<Result<Int, Error>>`. As with record fields, storing a caught concrete-family error as an `Error` payload requires explicit `error.wrap`. There are no guards, nested patterns, or partial patterns in this slice. Choices cannot be printed or compared directly; match them and print or compare the selected payload values.

Imported choices use the module-qualified type and alternative names, for example `model.Status.Pending` and `model.Status.Failed("offline")`. All alternatives share their parent type's module visibility. A module's public signatures cannot expose a private choice type.

Related guides: [records and named types](records.html), [values and bindings](values.html), and [results and error handling](results.html).
