---
title: Text operations
description: Reference Zhex string operations, integer formatting, Unicode behavior, indexing, and replacement.
section: Standard library
order: 70
---

Zhex validates strings as UTF-8. Text length, reversal, indexing, and slicing use Unicode code points rather than bytes. In the signatures below, `Result<T, E>` describes a built-in return value; programs cannot write this type in a word signature or define new result types.

## Formatting and parsing

| Signature | Behavior |
| --- | --- |
| `int.to-string(Int) -> String` | Convert an integer to signed decimal text. |
| `text.to-int(String) -> Result<Int, ParseError>` | On success, parse trimmed signed 64-bit decimal text; use `try` to unwrap it. |

There is no implicit conversion between `Int` and `String`.

## Inspect and transform text

| Signature | Behavior |
| --- | --- |
| `text.reverse(String) -> String` | Reverse Unicode code points. Combining marks are not grouped with their base character, and no normalization occurs. |
| `text.length(String) -> Int` | Count Unicode code points. |
| `text.line-count(String) -> Int` | Count lines. Empty text has 0 lines; a trailing newline does not add an extra line. |
| `text.word-count(String) -> Int` | Count fields separated by Unicode whitespace. |
| `text.upper(String) -> String` | Map text to uppercase using Unicode case mapping. |
| `text.lower(String) -> String` | Map text to lowercase using Unicode case mapping. |
| `text.trim(String) -> String` | Remove surrounding Unicode whitespace. |
| `text.contains(String, String) -> Bool` | Return whether the second string occurs in the first. |
| `text.index-of(String, String) -> Int` | Return the code-point position of the first occurrence, or `-1` if absent. |
| `text.slice(String, Int, Int) -> String` | Return code points from `start` up to but not including `end`. Negative `start` clamps to 0; `end` past the length clamps to the length; `start >= end` returns empty text. |
| `text.replace(String, String, String) -> String` | Replace every literal occurrence of the second string with the third. An empty search string is a runtime `replace-empty` error. |

Examples:

```zhex
print(text.length("héllo"))
print(text.index-of("héllo", "l"))
print(text.slice("héllo", 1, 3))
```

The output is `5`, `2`, and `él`. `text.replace` treats its search text literally, not as a pattern. The language does not currently provide normalization, grapheme-cluster operations, or case-insensitive comparison.

The [text-stats example](../examples.html#text-stats) combines line, word, and character counts with file reading.
