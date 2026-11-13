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
bad.zhex:1:1: print does not support type Result<String, InputError>
    print(read-line("Name: "))
          ^^^^^^^^^^^^^^^^^^^
hint: this value is a Result; add postfix try to unwrap it, as in text.to-int("N: ") try
```

Type errors may say where a value came from. This can point to the operation that produced an unexpected result. Hints describe a specific action only when the checker knows a fix.

The [error catalog](../errors.html) lists stable built-in diagnostic codes and their meanings. Codes supplied by `error.new` and `error.wrap` are application data and do not each have a catalog entry. On a language error, `zhex run file.zhex --report=json` emits a JSON object with `code`, `severity`, `file`, `line`, `column`, `message`, and `hint`. Checker diagnostics with an expression span can also include `span_start_column` and `span_end_column`; runtime diagnostics do not include these fields. A successful run emits no report.

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

## Inspect a returned error

An error bound by `catch` is an `Error` value for inspection. The built-in accessors are:

| Operation | Result |
| --- | --- |
| `error.code(Error)` | Stable error code as a `String`. |
| `error.message(Error)` | Human-readable message as a `String`. |
| `error.operation(Error)` | Operation that first produced the error. |
| `error.file(Error)` | Origin source filename. |
| `error.line(Error)` / `error.column(Error)` | Origin line and column as `Int`s. |
| `error.detail(Error, key)` | Documented detail value as a `String`, or `""` if absent. |
| `error.has-cause(Error)` | Whether a wrapped cause exists. |
| `error.cause(Error)` | A result containing the cause, or `error-no-cause`. |

For example, a handler can branch on the stable code rather than message wording:

```zhex
let value = text.to-int("bad") catch problem
    print(error.code(problem))
    print(error.operation(problem))
    0
print(value)
```

Use `error.detail` for structured fields such as a failed file path and normalized reason (`path`, `reason`), a bad buffer index (`index`, `line-count`), or an out-of-range argument (`index`, `argument-count`). File open reasons are `not-found`, `permission-denied`, or `other`; read reasons are `invalid-utf8`, `closed`, `read-failed`, or `end-of-file`. Missing detail keys return the empty string. `error.new(code, message)` creates a cause-less application error at that call site. `error.wrap(code, message, cause)` converts an existing error to the common `Error` family while preserving its origin and cause. Use `error.cause(problem) try` when a cause is required, or catch `error-no-cause` when it is optional.

`Error` is the common built-in error type, not a user-defined record. Result error families remain `InputError`, `ParseError`, `FileError`, `BufferError`, and `Error`; when a word changes a concrete family to `Error`, the conversion through `error.wrap` is explicit.

## Report handled and unhandled failures

An application may install a named reporter before its fallible calls:

```zhex
word report(problem: Error) -> Unit
    print(error.code(problem))

error.set-reporter(report)
```

The callback receives an `Error`. Call `error.report(problem)` in a `catch` body to report a failure the program handled. If a failure instead escapes the program, the runtime calls the reporter once before writing the normal diagnostic. Propagation through several `try` layers does not call it at each layer. Reporting the same occurrence explicitly and then allowing it to escape does not report that occurrence twice.

The reporter runs synchronously. Its failure cannot replace the original error, and reporting is suppressed while the callback itself runs. Nothing is sent to a network or external service by default; the program defines what the callback does. Error messages and details can include input or paths, so select and sanitize fields deliberately.
