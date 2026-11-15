---
title: Standard library direction
description: What ships today, how operations are named, and the proposed example-driven growth plan.
section: Standard library
order: 69
---

Zhex ships a small host library for text, typed collections, JSON, files, buffers, console input/output, errors, and temporal values. The <a href="https://github.com/desireco/zhex/blob/main/docs/standard-library-plan.md" target="_blank" rel="noopener">complete current inventory and research</a> lists all 95 registered host names and their test/reference evidence. Constructors, operators, bracket access, and REPL commands are separate facilities.

A dot identifies an operation's owner or domain: `text.trim`, `list.length`, `file.read-all`. Compound names use hyphens. Flows pass their value as the first argument. Conversions are explicit, as in `int.to-string`.

Text positions count Unicode code points. Collections have shared identity and guard active traversal against mutation. Ordinary computations return ordinary values; external input and parsing failures use explicit Results. See the [text](text.html), [collections](collections.html), [files](files.html), [results](results.html), and [temporal](temporal.html) references for implemented behavior.

## Proposed next work

The research compares Go, Ruby, Python, Gleam, Elm, and Haskell. Its recommendation is to validate a few additions through a line-filter/report program, then grow reusable collection composition:

1. Literal text split/join and prefix/suffix checks (#57), then writing a report to a file (#58).
2. Collection map/filter/fold using existing function values, with explicit callback order and aliasing rules (#60).
3. Short-circuit search (#61) and stable copied sorting (#62); fallible traversal is a separate design step.
4. Paths, environment configuration, and other host capabilities only when a concrete program needs them.

These additions are **proposals, not available APIs** — except #57's literal text operations (`text.split`, `text.join`, `text.starts-with`, `text.ends-with`), which shipped into the [text reference](text.html), and #58's `file.write(contents, path, append = false)`, which shipped into the [files reference](files.html) with built-in trailing constant defaults; and #60/#61's list callback words (`list.map`, `list.filter`, `list.fold`, `list.find`, `list.any`, `list.all`), which shipped into the [collections reference](collections.html) with ordinary function-value callbacks. The <a href="https://github.com/desireco/zhex/blob/main/docs/standard-library-plan.md" target="_blank" rel="noopener">research and draft stories</a> record proposed signatures, dependencies, error policies, tests, and deferred capabilities. The owner approved filing <a href="https://github.com/desireco/zhex/issues/30" target="_blank" rel="noopener">stories #57–64</a> for one-by-one review; they were initially filed in Inbox and then moved to Todo at the owner's request, and #57, #58, #60, and #61 have since shipped. The existing Game of Life and edit-distance examples already run; they do not depend on these proposals.

The current module system loads sibling files. This plan does not add installed standard modules, generic user words, implicit conversions, or a package manager. Track discussion in <a href="https://github.com/desireco/zhex/issues/30" target="_blank" rel="noopener">issue #30</a>.
