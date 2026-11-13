---
title: Modules
description: Split programs across sibling files with use, qualified calls, and underscore-private words.
section: Language
order: 60
---

`use name` loads `name.zhex` beside the file that contains the directive. Each imported file resolves its own imports beside itself. A module contains word and type declarations; top-level statements belong in the entry file.

`main.zhex` fragment:

```zhex
use math

print(math.gcd(48, 18))
```

The imported `math.zhex` can define `gcd` like this:

```zhex
word gcd(a: Int, b: Int) -> Int
    if b == 0
        a
    else
        gcd(b, a % b)
```

The complete two-file program lives in the repository. Its entry file imports the sibling `math.zhex`; run it from the repository root with:

```sh
zhex run examples/modules/main.zhex
```

## Call imported words

Use the module name to call an imported word: `math.gcd(a, b)`. A qualified word can also be a flow step if it takes one argument. Inside its own file, a module's words call each other by their bare names. Imported words do not see the importing file's words.

A module's own imports stay private to that module. To use one of those words directly, import that module by name in the entry file too. Separate modules can define words with the same name without colliding.

Imported record types are qualified with their module name, just like words. For example, after `use model`, annotate a value as `model.Customer` and construct it with `model.Customer("Ada", 36)`. The module's words use the local type name in their own signatures. Types beginning with `_` are module-private. Public word signatures and record fields cannot expose a private type, including through `List`, `Array`, or `Result`.

## Keep a word private

A word whose name begins with `_` is private to its module. It can be called by other words in that same file, but not from the importer. The checker reports an attempt to call it from outside.

`math.zhex` fragment:

```zhex
word gcd(a: Int, b: Int) -> Int
    if b == 0
        a
    else
        gcd(b, a % b)

word _normalize(n: Int) -> Int
    if n < 0
        0 - n
    else
        n
```

The leading underscore is the visibility rule; there is no separate export declaration.

## Resolution and errors

`use` is allowed only at file top level. Repeating the same import is a no-op. A missing sibling file reports the searched path. Import cycles are errors, and the module names `text`, `int`, `buffer`, `file`, `console`, `error`, `list`, and `array` are reserved for built-in operations. An imported file with top-level statements is rejected.

All imported modules and the entry file are checked before execution. Module errors point to the module's own source. The two-file [modules example](../examples.html#modules/main) shows a complete program.

In the REPL, `use math` resolves `math.zhex` from the working directory and adopts its qualified words for the session. See [REPL](repl.html).
