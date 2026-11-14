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

The REPL checks and runs each submitted entry. Defined words, values, and adopted modules persist across entries. Use Ctrl-D or call the preloaded ordinary word `bye()` to exit with status 0. `bye()` appears in `words` and follows the usual word-redefinition rules; it is available only in the REPL.

## Enter a word

Calls use parentheses: enter `print(1 + 1)` to print `2`. The REPL does not accept the shell-like form `print 1 + 1`.

In an interactive terminal, the continuation editor inserts four spaces after a block header. It carries the current indentation into nested blocks. Enter submits a line; a line containing only the suggested indentation is blank and submits the entry. Tab inserts four spaces at the cursor, and Backspace removes one character; press it four times to dedent one level before `else`:

```text
Zhex REPL — end multi-line entries with a blank line; bye() or Ctrl-D exits.
zhex> print(1 + 1)
2
zhex> word positive(n: Int) -> Int
....>     if n > 0
....>         n
....>     else
....>         0
....>
zhex> flow 3
....>     positive
....>     print
....>
3
```

The spaces shown after `....>` are editable spaces supplied by the terminal editor. To enter `else`, press Backspace four times to remove one indentation level before you type it. The flow sends `3` to `positive`, then sends the result to `print`.

An `if`, `match`, or `flow` block can be the whole value expression on the right side of a binding or assignment. The REPL keeps collecting an entry after a `match` header:

```text
zhex> let clean = flow "  ADA  "
....>     text.trim
....>     text.lower
....>
zhex> clean
ada
zhex> let value = match Option.Some(3)
....>     case Option.Some(n)
....>         n
....>     case Option.None
....>         0
....>
zhex> value
3
```

The final value of each block supplies the binding. The block must be the entire right-hand side; it cannot be nested in a call or another expression.

While editing, Ctrl-C cancels the current entry and returns to the top-level prompt. Ctrl-D on an empty top-level prompt exits with status 0; in a pending entry, Ctrl-D on an empty or spaces-only continuation line discards it and exits. Bracketed paste keeps the pasted source indentation, including literal tabs; tabs and non-ASCII whitespace outside strings or comments remain source errors. These editing conveniences apply only when both input and output are interactive terminals. Piped entries keep the existing line-based behavior and must include their own four-space indentation.

When a program uses `read-line`, the terminal reader also supplies its input. It reads the typed line as data, without source auto-indentation or Tab expansion.

An open `()`, `[]`, or `{}` keeps an entry open across lines, including blank and comment-only lines. Closing the delimiter submits a completed top-level expression immediately. If the expression is inside a pending indented block, the block still needs a blank line to submit. The same delimiter rules apply to piped source; an unclosed delimiter at piped EOF is an error. Angle brackets in types do not continue an entry.

## Results and errors

A bare ordinary expression echoes its value; `Unit` has no visible value. A bare `Result` is rejected and must be handled with `try` or `catch` in the same entry. `let` and `print` entries do not echo a result.

Checker and runtime errors print an annotated diagnostic, and the session remains open. A checker error adopts none of the entry's definitions or values. If a runtime error occurs, effects that already happened in that entry remain.

## Inferred word signatures

The REPL lists inferred parameter, return, and error types under `words`. A parameter can be inferred from its body or from a call in the same entry; later entries cannot add constraints to an already checked definition:

```text
zhex> word parse(s: String)
....>     text.to-int(s) try
....>
zhex> words
bye() -> Unit
parse(s: String) -> Result<Int, ParseError>
zhex> parse("9") try
9
zhex> word identity(value)
....>     value
....> print(identity(7))
....>
7
zhex> words
bye() -> Unit
identity(value: Int) -> Int
parse(s: String) -> Result<Int, ParseError>
```

Defaults appear as `= ...` in a word's effective signature. Calls stay positional and can omit trailing defaulted parameters:

```text
zhex> word greet(name = "Ada")
....>     "Hello " + name
....>
zhex> words
bye() -> Unit
greet(name: String = ...) -> String
zhex> greet()
Hello Ada
```

`let` can shadow an earlier value silently. Redefining a word with the same effective signature replaces it and prints `redefined name`. A redefinition that changes a word's effective Result signature is rejected transactionally, so the previous word and its callers remain available. The `words` command lists signatures in name order; `words` is a command even if a user has a binding with that name.

After a compatible redefinition, callers already checked against the earlier word keep using that definition and its defaults. New entries use the replacement.

Ctrl-D exits with status 0; `bye()` does the same. A final input line without a newline still runs. `use name` resolves `name.zhex` in the working directory and adopts that module's qualified words for the rest of the session.

## Trace a session

`zhex repl --trace` streams JSONL step events to standard error while the REPL runs entries. The event file field is `repl`. See [diagnostics and tracing](diagnostics.html) for the event vocabulary.
