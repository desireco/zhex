---
title: Diagnostics and tracing
description: Read checker and runtime diagnostics, request JSON reports, and trace interpreter steps.
section: Tools
order: 120
---

The checker validates names, types, and word connections across the whole program before execution. A static error means no prompt is written and no input is consumed. Runtime errors happen during execution and may follow earlier output.

## Read a diagnostic

Text diagnostics start with `file:line:column: message`. The source line follows, with a caret under the failing expression. When the checker knows a concrete fix, it adds a `hint:` line.

```text
bad.zhex:2:1: print does not support type Result<String, InputError>; line comes from read-line
    print(line)
          ^^^^
hint: this value is a Result; add postfix try to unwrap it, as in text.to-int("N: ") try
```

Type errors may say where a value came from. This can point to the operation that produced an unexpected result. Hints describe a specific action only when the checker knows a fix.

The [error catalog](../errors.html) lists stable diagnostic codes and their meanings. On a language error, `zhex run file.zhex --report=json` emits a JSON object with `code`, `severity`, `file`, `line`, `column`, `message`, and `hint`. Checker diagnostics with an expression span can also include `span_start_column` and `span_end_column`; runtime diagnostics do not include these fields. A successful run emits no report.

## Trace a run

Use `--trace` to see interpreter steps as JSON Lines on standard error:

```sh
zhex run examples/composition.zhex --trace
```

The event vocabulary is:

| Event | Data |
| --- | --- |
| `word-call` | Program word name, source file and line, and rendered arguments. Built-in calls do not emit this event. |
| `binding` | Name, rendered value, and source file and line. A result binding renders as `ok("value")` or `err("message")`. |
| `flow-step` | Word name, the value passed to it, and source file and line. |
| `word-return` | Program word name, returned value, and the word's source file and line. |

Values use the checked type for rendering: integers as digits, booleans as `true` or `false`, strings verbatim, Unit as an empty string, buffers as `buffer(n lines)`, and files as `file("path")`. Module word names are qualified. REPL traces use `"file":"repl"`.

Tracing can be combined with `--report=json`: step events go to standard error before the final report. A trace write failure does not fail a run. Without `--trace`, normal program output is unchanged.
