---
title: "TL;DR — Zhex in a minute"
description: What Zhex is, what a program looks like, and where the project stands.
section: Start
order: 0
---

Zhex is an experimental, Forth-inspired programming language built around small named operations called **words**. It combines that idea with named values, familiar expressions, and explicit control flow. A Go interpreter checks a program's names and types before running it.

The aim is readable composition: write a small operation, give important values names, and connect operations so the movement of data is visible.

## A small program

Save this as `square.zhex`:

```zhex
word square(n: Int) -> Int
    n * n

let number = 7

flow number
    square
    print
```

After [building Zhex and adding it to your path](getting-started.html), run:

```sh
zhex run square.zhex
```

It prints `49`. A word's final expression supplies its result. Here, `flow` passes `7` to `square`, then passes `49` to `print`. Blocks use four spaces of indentation. You can also call words directly; flows are useful when a sequence reads naturally.

## What matters

Types can be inferred where the program supplies enough information. There are no implicit conversions: turning an integer into text is an explicit operation. Fallible input and parsing use Results; `try` propagates a failure and `catch` supplies local recovery. The checker rejects incompatible composition before execution, while runtime checks handle problems such as overflow and invalid indexes.

The working language includes records, choices, lists, multidimensional arrays, maps, typed JSON, dates and time zones, text/file reading, and sibling-file modules. The [examples](../examples.html) include text statistics, Game of Life, and edit distance. An interactive session is available with `zhex repl`.

## Where it stands

Zhex is still evolving; syntax and APIs may change. Today it runs through the Go interpreter. There is no native compiler, package manager, floating-point type, or generic user-defined word support. Standard-library expansion is being planned through concrete programs.

Start with [Getting started](getting-started.html), explore the [examples](../examples.html), or read the [roadmap](../roadmap.html) to see what is shipped and what remains proposed.
