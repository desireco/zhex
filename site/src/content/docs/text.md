---
title: Text operations
description: Reference Zhex string operations, integer formatting, Unicode behavior, indexing, and replacement.
section: Standard library
order: 70
---

Zhex validates strings as UTF-8. Text length, reversal, indexing, and slicing use Unicode code points rather than bytes. In the signatures below, `Result<T, E>` describes a built-in return value. User words may omit a return annotation when the body determines one concrete success type; an escaping `try` makes the checker infer the effective Result signature. Explicit return and `Result<T, E>` word signatures remain supported; user-defined error types are not.

## Formatting and parsing

| Signature | Behavior |
| --- | --- |
| `int.to-string(Int) -> String` | Convert an integer to signed decimal text. |
| `text.to-int(String) -> Result<Int, ParseError>` | On success, parse trimmed signed 128-bit decimal text; use `try` to propagate failure or `catch` to recover. |

There is no implicit conversion between `Int` and `String`. One exception exists — the string-context Int exception: `+` concatenates when exactly one operand is `Int` and the other is `String` (the Int renders as `int.to-string` does, chains associate left to right, and two `Int` operands still add numerically), and `print` accepts an `Int` directly. `Bool` and every other type stay strict in these positions, and no other position converts.

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
| `text.split(String, String) -> List<String>` | Split on every literal occurrence of the second string, preserving leading, repeated, and trailing empty fields with no trimming. An empty separator splits into single code points; empty text with an empty separator yields an empty list, and empty text with a nonempty separator yields one empty field. A result beyond 1,000,000 elements fails `collection-limit` before the list is materialized. |
| `text.join(List<String>, String) -> String` | Join the elements with the separator between each pair. An empty list joins to empty text, and an empty list literal is accepted as `List<String>`. |
| `text.to-code-points(String) -> List<Int>` | Return the code points as a fresh `List<Int>` in order. A result beyond 1,000,000 elements fails `collection-limit` before the list is materialized. |
| `text.from-code-points(List<Int>) -> String` | Rebuild text from a list of code points. A code point that is not valid Unicode — negative, above U+10FFFF, or a surrogate half — is a runtime `text-code-point` error. |
| `text.starts-with(String, String) -> Bool` | Return whether the first string starts with the second, compared literally; an empty prefix matches. |
| `text.ends-with(String, String) -> Bool` | Return whether the first string ends with the second, compared literally; an empty suffix matches. |
| `text.repeat(String, Int) -> String` | Return `n` concatenated copies of the text. A count of `0` returns empty text; a negative count is a runtime `text-repeat-count` error. |
| `text.pad-left(String, Int, String = " ") -> String` | Pad to `width` code points by prefixing `fill`. Omitting `fill` pads with spaces. The fill must be exactly one code point (a runtime `text-pad-fill` error otherwise) and the width nonnegative (`text-pad-width`); a width at or below the text's length returns the text unchanged — padding never truncates. |
| `text.pad-right(String, Int, String = " ") -> String` | Pad to `width` code points by suffixing `fill`, with the same fill, width, and never-truncates rules as `text.pad-left`. |
| `text.wrap(String, Int) -> List<String>` | Greedily wrap on spaces: a line breaks before the first word that no longer fits, an over-long word overflows alone rather than breaking mid-word, and empty or spaces-only input wraps to one empty line. Width counts code points — a monospace approximation, not a typographic measure. A result beyond 1,000,000 lines fails `collection-limit` before the list is materialized. |
| `text.insert(String, Int, String) -> String` | Place `inserted` at the code-point index. Insertion points run `0` through the length inclusive — index == length appends — and an out-of-range index clamps into range the way `text.slice`'s bounds do. |
| `text.remove(String, Int, Int) -> String` | Remove the code points from `start` up to but not including `end`, with `text.slice`'s clamping: negative `start` clamps to 0, `end` past the length clamps to the length, and an empty or inverted range returns the text unchanged. |

Examples:

```zhex
print(text.length("héllo"))
print(text.index-of("héllo", "l"))
print(text.slice("héllo", 1, 3))
print(text.pad-left("7", 3, "0"))
print(text.insert("ac", 1, "b"))
print(text.join(text.wrap("aa bb cc", 5), "|"))
```

The output is `5`, `2`, `él`, `007`, `abc`, and `aa bb|cc`. `text.replace` treats its search text literally, not as a pattern. The language does not currently provide normalization, grapheme-cluster operations, or case-insensitive comparison.

The [text-stats example](../examples.html#text-stats) combines line, word, and character counts with file reading.
