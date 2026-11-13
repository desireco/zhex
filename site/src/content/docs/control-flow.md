---
title: Control flow
description: Use typed if branches and bounded range or streaming file loops in Zhex.
section: Language
order: 40
---

`if` selects a branch, and `for` repeats a block. Each block has lexical scope. Conditions, branch results, and loop bounds are checked before the program runs.

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

When an `if` is used as a value, its branches must return the same type. Without `else`, the missing branch supplies `Unit`, so every present branch must also return `Unit`; a value-producing `if` needs an `else`. Each branch has its own scope.

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

The other loop form is `for line in file.lines(path)`. It reads one UTF-8 line at a time and binds each chomped line as an immutable `String`:

```zhex
for line in file.lines("examples/text-stats-sample.txt")
    print(text.upper(line))
```

Run that program from the repository root so the sample path resolves.

The loop closes the file when it finishes, including when the body fails. An empty file runs no iterations. A final line without a newline still runs. See [files](files.html) for line ending and failure details.

Loops and branches can contain calls and flows. See [words and flows](words-and-flows.html) and [values](values.html) for the related type and scope rules.
