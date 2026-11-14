---
title: Calls and defaults
description: Call words with parentheses or simple comma-separated arguments, and understand where the shorthand applies.
section: Language
order: 32
---

Zhex accepts both parenthesized calls and a short form for simple word calls. The parenthesized form remains available everywhere it was before.

```zhex
word greet name: String = "Ada"
    "Hello, " + name

word add(left: Int, right: Int) -> Int
    left + right

word seven
    7

let default-greeting = greet
let named-greeting = greet "Grace"
let parenthesized-sum = add(1, 2)
let bare-sum = add 1, 2
let answer = seven

print default-greeting
print named-greeting
print parenthesized-sum
print bare-sum
print answer
print 1 + 1
print(add(-1, 2))

let values = [8, 5]
print(values[0] - values[1])
```

This prints `Hello, Ada`, `Hello, Grace`, `3`, `3`, `7`, `2`, `1`, and `3`, each on its own line. The complete source is also available as [calls.zhex](../examples.html#calls).

## Word headers and calls

A word header may write parameters without parentheses: `word add left: Int, right: Int`. The existing form `word add(left: Int, right: Int)` remains valid. A zero-parameter word may be written without empty parentheses:

```zhex
word answer
    42

let value = answer
print value
```

At a complete expression boundary, an accessible word name with no following arguments calls the word with zero supplied arguments. A local value with the same name resolves as a value. Thus `let answer = answer` can call the word only when there is no binding named `answer` already in scope.

For one or more arguments, commas mark the argument boundaries: `add 1, 2` is equivalent to `add(1, 2)`. Each comma-separated argument is an ordinary expression. The short form can be a complete statement, a full right-hand side of a binding or assignment, the final expression in a word body, or a flow step. It does not turn whitespace into a general argument separator: `add 1 2` is invalid.

In a flow, the current value is the first argument and the short-form arguments follow it:

```zhex
word add(left: Int, right: Int) -> Int
    left + right

flow 4
    add 2
    print
```

The `add 2` step is equivalent to `add(4, 2)`, so this prints `6`.

## Explicit partial application

Use `_` in an argument position to leave that argument for later. Each hole becomes a required parameter of the resulting function, in left-to-right order. Every other required argument must be supplied now: `add 5` is a missing-argument error, not an automatic partial. `_` is a slot on its own, not an anonymous function body. Supplied arguments are evaluated once, from left to right, and captured when the partial is created:

```zhex
word add left: Int, right: Int -> Int
    left + right

let plus-five = add _, 5
let five-plus: (Int) -> Int = add 5, _
print plus-five(7)
let result = five-plus 7
print result
```

This prints `12` twice. The first partial leaves the left argument open; the second leaves the right argument open. The optional `(Int) -> Int` annotation describes the one argument still required. A function value can use either call form, and explicit parentheses remain useful in nested expressions.

A partial can also be a flow step, or keep a defaulted parameter deferred until invocation:

```zhex
word add left: Int, right: Int -> Int
    left + right

word append-and-count(item: Int, values: List<Int> = []) -> Int
    list.append(values, item)
    list.length(values)

let plus-five = add _, 5
let add-to-fresh-list = append-and-count _

print add-to-fresh-list(1)
let next-length = add-to-fresh-list 2
print next-length

flow 7
    plus-five
    print
```

This prints `1`, `1`, and `12`. Each invocation gets a new empty list because the default is evaluated when the partial is called. The [partial application example](../examples.html#partials) also shows a captured integer value: rebinding its original `var` does not change the value the partial captured.

## Built-in calls and function types

Built-ins with a fixed signature can also be partially applied. A contextual function annotation can resolve an overloaded operation such as `print`:

```zhex
let show: (Int) -> Unit = print _
show 7
```

This prints `7`. The annotation resolves `print`'s overload here. Whole-program call constraints can also do so without an annotation, as in `let show = print _` followed by `show 7`; if neither annotation nor call sites determine a type, the checker reports ambiguity. A partial of `text.replace` can infer its remaining argument types from the operation and supplied arguments. The [partial application example](../examples.html#partials) also includes `list.append _, 1`.

Collection intrinsics that normally return plain values keep that mode when partially applied. The function created by `list.append _, 1` returns `Unit` when invoked; adding `try` at invocation cannot switch it to a checked `Result`. To make a fallible function, put the intrinsic's checked form inside an ordinary word and partially apply that word:

```zhex
word append-checked(values: List<Int>, item: Int)
    list.append(values, item) try

let append-one = append-checked _, 1
let values: List<Int> = []
append-one(values) try
print(list.length(values))
```

Here the intrinsic's `try` selects its checked form inside `append-checked`, so the partial's invocation returns a `Result` that its caller handles.

Record and choice constructors are not partial targets. Loop-only `range` and `file.lines`, special reporter registration `error.set-reporter`, and the non-returning `console.exit` and `console.fail` are not partial targets either; call them directly where they are valid.

Function types are monomorphic arrows with a fixed argument tuple, such as `(Int) -> Unit` or `(String, Int) -> String`. They can annotate bindings and parameters, and can be passed to words, returned from words, used in flows, or stored in lists, records, and choices. A function's result type may be a `Result` to describe a failure that will occur when it is invoked; a `Result` cannot be an argument or a captured value. Function values cannot be printed, compared directly, or encoded as JSON. Caught concrete errors follow the usual rule: wrap one with `error.wrap` before capturing it for a function that accepts common `Error`.

## Handling a fallible call

A trailing `try` or `catch` at a complete bare-call boundary handles that call's result. To handle a fallible argument separately, keep the outer call parenthesized and put `try` on the argument:

```zhex
word add(left: Int, right: Int) -> Int
    left + right

let parsed = text.to-int "4" try
let total = add(1, text.to-int("2") try)
let fallback = text.to-int "bad" catch problem
    3

print parsed
print total
print fallback
```

This prints `4`, `3`, and `3`. In the first binding, `try` guards the whole bare `text.to-int` call. In the second, it guards only the parenthesized call argument. A `catch` block can also supply a fallback for a bare call.

A partial of a fallible word does not run that word when the partial is created. Its function result type carries the `Result`, so handle the failure at invocation:

```zhex
word parse(raw: String)
    text.to-int(raw) try

let parse-later = parse _
let value = parse-later "8" try
let fallback = parse-later "bad" catch problem
    print(error.code(problem))
    3

print value
print fallback
```

This prints `8`, `text-to-int`, and `3`. `try` on an explicitly supplied capture argument belongs to partial creation; the partial's own `try` is invalid because it is not a `Result` until invocation.

## Keep parentheses for nesting and ambiguity

Arguments retain normal operator precedence. For example, `print 1 + 1` calls `print` with the result of `1 + 1`. When a call is nested in another expression, keep the nested call parenthesized, as in `print(add(1, 2))`. Whitespace alone does not tell the parser which nested call owns the arguments.

Parentheses also make ambiguous first arguments clear. Write `display(-1)` for a negative argument, or `first([7, 8])` for a list literal argument. Existing indexing remains indexing: `values [0]` still means the first element of `values`, not a call passing `[0]`.

Named arguments remain specific to record constructors. Word arguments are positional, including in the short form. A word default can supply omitted trailing arguments; see [words and flows](words-and-flows.html#default-parameters) for evaluation order and scope.
