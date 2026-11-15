---
title: Environment configuration
description: Read settings from the process environment and load KEY=VALUE files with first-assignment-wins.
section: Standard library
order: 105
---

The `env` words read configuration from the process environment. `env.get` answers one name; `env.load` reads a `.env`-style file into the environment so one `env.get` shape covers every setting:

```zhex
match env.get("ZHEX_REPORT_TITLE")
    case Option.Some(title)
        print("title: " + title)
    case Option.None
        print("title: unset")
```

| Signature | Result and behavior |
| --- | --- |
| `env.get(name: String) -> Option<String>` | Total, never fails. An unset name is `Option.None`; a name set to the empty string is `Option.Some("")`. |
| `env.load(path: String = ".env") -> Result<Unit, Error>` | Reads KEY=VALUE lines into the process environment. |

Both words are partially appliable like other value-returning built-ins; a partial of `env.load` keeps its `.env` default deferred, the `file.write` `append` rule.

## Reading one name

`env.get` never fails and returns no `Result`: missing and empty stay distinct through the built-in `Option`. An environment variable set to the empty string answers `Option.Some("")`, and a name that is not set at all answers `Option.None` — the two states never collapse, so a match is the natural reader.

## Loading a file

`env.load(path)` reads the file line by line and sets each `KEY=VALUE` into the process environment, returning `Result<Unit, Error>` in the common `Error` family. Omitting the path loads `.env` in the working directory. The parse rules:

- Blank lines and whole-line `#` comments are skipped after leading trim.
- Whitespace around the key and the value is trimmed.
- One matching pair of surrounding single or double quotes on the value is stripped: `"spaced value"` and `'single quoted'` load without their quotes.
- `KEY=` sets the empty string.
- Any other line — a missing `=`, an empty key, or a key containing spaces — fails `env-parse` naming the line number and the offending text.

The first assignment wins: a key already in the environment, or set by an earlier line of the same file, keeps its value and is never overridden. Lines before a malformed one are already applied and stay applied, like the file words — `env.load` makes a single pass. There is no `export` prefix, no inline comments, and no escape processing.

Failures use the stable codes `env-read` (the file cannot be read, naming the path and the operating system's reason) and `env-parse` (a malformed line). Handle either with `try` or `catch`.

See the [env-config example](../examples.html#env-config), which writes a `report.env` fixture, loads it, reads its settings back through Option matches, and cleans up.
