---
title: Standard library direction
description: What ships today, how operations are named, and the proposed example-driven growth plan.
section: Standard library
order: 69
---

Zhex ships a small host library for text, typed collections, JSON, files, buffers, console input/output, errors, and temporal values. The [complete current inventory and research](https://github.com/desireco/zhex/blob/main/docs/standard-library-plan.md) lists all 84 registered host names and their test/reference evidence. Constructors, operators, bracket access, and REPL commands are separate facilities.

A dot identifies an operation's owner or domain: `text.trim`, `list.length`, `file.read-all`. Compound names use hyphens. Flows pass their value as the first argument. Conversions are explicit, as in `int.to-string`.

Text positions count Unicode code points. Collections have shared identity and guard active traversal against mutation. Ordinary computations return ordinary values; external input and parsing failures use explicit Results. See the [text](text.html), [collections](collections.html), [files](files.html), [results](results.html), and [temporal](temporal.html) references for implemented behavior.

## Proposed next work

The research compares Go, Ruby, Python, Gleam, Elm, and Haskell. Its recommendation is to validate a few additions through a line-filter/report program, then grow reusable collection composition:

1. Literal text split/join and prefix/suffix checks, followed by writing a report to a file.
2. Collection map/filter/fold using existing function values, with explicit callback order and aliasing rules.
3. Short-circuit search and stable copied sorting; fallible traversal is a separate design step.
4. Paths, environment configuration, and other host capabilities only when a concrete program needs them.

These additions are **proposals, not available APIs**. The [research and draft stories](https://github.com/desireco/zhex/blob/main/docs/standard-library-plan.md) record proposed signatures, dependencies, error policies, tests, and deferred capabilities. Agreement comes before implementation tickets. The existing Game of Life and edit-distance examples already run; they do not depend on these proposals.

The current module system loads sibling files. This plan does not add installed standard modules, generic user words, implicit conversions, or a package manager. Track discussion in [issue #30](https://github.com/desireco/zhex/issues/30).
