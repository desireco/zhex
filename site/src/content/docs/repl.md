---
title: REPL
description: Use the interactive Zhex session to define words, keep values, and inspect signatures.
section: Tools
order: 110
---

Start an interactive session with:

```sh
zhex repl
```

The REPL checks and runs each submitted entry. Defined words, values, and adopted modules persist across entries.

## Enter a word

A word body needs an indented block. The REPL uses a continuation prompt while it is waiting for that block. Submit a multi-line entry with a blank line:

```text
zhex> word greet(name: String) -> String
....>     "Hello, " + name + "!"
....>
zhex> greet("Ada")
Hello, Ada!
```

Only a missing indented block continues an entry. Parentheses and incomplete expressions do not continue a line. Statements do not span lines.

## Results and errors

A bare ordinary expression echoes its value; `Unit` has no visible value. A bare `Result` is rejected and must be handled with `try` or `catch` in the same entry. `let` and `print` entries do not echo a result.

Checker and runtime errors print an annotated diagnostic, and the session remains open. A checker error adopts none of the entry's definitions or values. If a runtime error occurs, effects that already happened in that entry remain.

## Inferred word signatures

When a word's body lets a built-in failure escape through `try`, the REPL lists its effective Result signature, even though the source declares the success type:

```text
zhex> word parse(s: String) -> Int
....>     text.to-int(s) try
....>
zhex> words
parse(s: String) -> Result<Int, ParseError>
zhex> parse("9") try
9
```

`let` can shadow an earlier value silently. Redefining a word with the same effective signature replaces it and prints `redefined name`. A redefinition that changes a word's effective Result signature is rejected transactionally, so the previous word and its callers remain available. The `words` command lists signatures in name order; `words` is a command even if a user has a binding with that name.

Press Ctrl-D to exit with status 0. A final input line without a newline still runs. `use name` resolves `name.zhex` in the working directory and adopts that module's qualified words for the rest of the session.

## Trace a session

`zhex repl --trace` streams JSONL step events to standard error while the REPL runs entries. The event file field is `repl`. See [diagnostics and tracing](diagnostics.html) for the event vocabulary.
