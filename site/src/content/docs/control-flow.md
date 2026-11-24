---
title: Control flow
description: Use typed if branches and bounded range, streaming file, and while loops in Zhex.
section: Language
order: 40
---

`if` selects a branch, and `for` and `while` repeat a block. Each block has lexical scope. Conditions, branch results, and loop bounds are checked before the program runs.

## Choose with `if`

Conditions must have type `Bool`. Add any number of `elif` branches and an optional `else`:

```zhex
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

An `if` whose value is used must have branches that all return the same type. Its value is used when it is the last line of a word that returns a value, as in `fizzbuzz`, or when it stands after `let`, `var`, or `=`. Without `else`, the missing branch supplies `Unit`, so every present branch must also return `Unit`; an `if` that produces a value needs an `else`.

An `if` on its own line drops what its branches return, because nothing reads it. There each branch may end with any type, and `else` is optional. Such a line is an ordinary line: every line of a program's top level and of a loop body is one.

```zhex
let xs: List<Int> = []
for n in range(0, 4)
    if n > 1
        list.append(xs, n)
print(xs[0])
print(xs[1])
```

This prints `2` and `3`. `list.append` returns the list it changed, and no other branch has to return the same type.

The last line of a word declared `-> Unit` is an ordinary line too, like the word's other lines:

```zhex
word seed(xs: List<Int>) -> Unit
    if list.length(xs) == 0
        list.append(xs, 1)

let xs: List<Int> = []
seed(xs)
print(list.length(xs))
```

This prints `1`. Without `-> Unit` the checker rejects `seed`: the word would have no declared result, so its last line would be the word's result, and that line returns the list on one path and `Unit` on the other. A word that returns nothing declares `-> Unit`.

An `if` whose branches return different types fails with `branch result types must match` wherever its value is used, including an `if` typed at the REPL prompt: the prompt echoes its value. A branch cannot end in a bare `Result` on any line; handle it with `try` or `catch`. Each branch has its own scope.

## Repeat a range

`range(start, end)` iterates upward from `start` and stops before `end`. Both bounds are evaluated once. If `start >= end`, the loop body does not run. The iteration name is an immutable `Int`, local to the loop.

```zhex
var total = 0
for n in range(1, 5)
    total = total + n
print(total)
```

This prints `10`. The loop itself has type `Unit`; it does not produce a collection or a final value. `range` is available only in this loop syntax.

## Stream a file's lines

The other built-in loop forms are `for line in file.stream(path)` and collection iteration. The file form reads one UTF-8 line at a time and binds each chomped line as an immutable `String`:

```zhex
for line in file.stream("examples/text-stats-sample.txt")
    print(text.upper(line))
```

Run that program from the repository root so the sample path resolves.

The loop closes the file when it finishes, including when the body fails. An empty file runs no iterations. A final line without a newline still runs. See [files](files.html) for line ending and failure details.

Lists and arrays can also be traversed with `for value in collection`; see [collection iteration](iteration.html) for index and coordinate bindings, traversal order, and mutation rules.

## Repeat while a condition holds

`while condition` repeats an indented body for as long as its condition holds. The condition must have type `Bool` and is evaluated before each pass, so a false condition runs no passes. A `var` updated in the body drives termination:

```zhex
var n = 1
while n <= 4
    print(n)
    n = n + 1
```

This prints `1` through `4`. The loop is a statement of type `Unit`; it cannot be the right-hand side of a binding or assignment. There is no `break` or `continue` — change the condition, or the values it reads, to leave the loop. A `while` may run forever by design: there is no iteration limit, and the word-call depth limit does not apply to iterations.

Loops and branches can contain calls and flows. See [words and flows](words-and-flows.html) and [values](values.html) for the related type and scope rules.
