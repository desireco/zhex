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

For one or more arguments, commas mark the argument boundaries: `add 1, 2` is equivalent to `add(1, 2)`. The short form can be a complete statement, a full right-hand side of a binding or assignment, the final expression in a word body, or a flow step. It does not turn whitespace into a general argument separator: `add 1 2` is invalid.

In a flow, the current value is the first argument and the short-form arguments follow it:

```zhex
word add(left: Int, right: Int) -> Int
    left + right

flow 4
    add 2
    print
```

The `add 2` step is equivalent to `add(4, 2)`, so this prints `6`.

## Keep parentheses for nesting and ambiguity

The short form consumes one ordinary argument expression. For example, `print 1 + 1` calls `print` with the result of `1 + 1`. When a call is nested in another expression, keep the nested call parenthesized, as in `print(add(1, 2))`. Whitespace alone does not tell the parser which nested call owns the arguments.

Parentheses also make ambiguous first arguments clear. Write `display(-1)` for a negative argument, or `first([7, 8])` for a list literal argument. Existing indexing remains indexing: `values [0]` still means the first element of `values`, not a call passing `[0]`.

Named arguments remain specific to record constructors. Word arguments are positional, including in the short form. A word default can supply omitted trailing arguments; see [words and flows](words-and-flows.html#default-parameters) for evaluation order and scope.
