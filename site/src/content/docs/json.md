---
title: JSON
description: Decode typed JSON with explicit defaults and encode supported Zhex values.
section: Standard library
order: 110
---

The `json` operations convert between JSON text and checked Zhex values. Decoding requires a value of the target type to supply defaults for missing record fields:

```zhex
type Settings = record
    retries: Int
    verbose: Bool

let settings = json.decode("{\"verbose\":true}", Settings(3, false)) try
print(settings.retries)
print(settings.verbose)
print(json.encode(settings) try)
```

Output:

```text
3
true
{"retries":3,"verbose":true}
```

`json.decode(raw, defaults)` returns `Result<T, Error>`, with `T` inferred from the defaults value. Every missing record field uses its corresponding default, including fields in nested records when a matching default record is present. A supplied field replaces that default; present lists and maps replace their defaults instead of merging. Each decode gets fresh copies of mutable default values, while aliases within one decoded value still refer to the same copied collection.

| Operation | Signature | Behavior |
| --- | --- | --- |
| `json.decode(raw, defaults)` | `(String, T) -> Result<T, Error>` | Decode JSON using the defaults value to determine `T` and fill missing record fields. |
| `json.encode(value)` | `(T) -> Result<String, Error>` | Encode a supported value as compact JSON. |

The decoder checks every supplied value. It rejects unknown fields, duplicate JSON object keys, wrong types, and missing fields with no default; it never substitutes a default for an invalid supplied value. A JSON `null` is valid only for `Option<T>` and produces `Option.None`. An omitted `Option<T>` field uses the default value, so `null` and omission have different meanings. `Option.Some(record)` can provide a default record for a non-null Option value; `Option.None` cannot provide inner defaults. Lists and maps of records do not provide a per-item default record: each supplied record item must include its required fields.

## Supported values

JSON supports `Int`, `Bool`, `String`, records, `List<T>`, `Map<String, T>`, and `Option<T>`, including nested combinations of those types. Direct `Option<Option<T>>` is rejected because `null` would make the nested representation ambiguous. `json.encode(value)` returns `Result<String, Error>` and writes compact JSON: record fields follow declaration order and map entries follow insertion order. `Option.Some(value)` encodes as its contained value; `Option.None` encodes as `null`.

Integers must use an integer-form JSON number within the signed 64-bit range. Fractions and exponent notation are rejected, even when their mathematical value is an integer; there is no rounding or floating-point conversion.

The current codec does not support `Unit`, Results, buffers, files, arrays, choices other than `Option`, or maps with non-string keys. These unsupported types are rejected by the checker before execution. JSON nesting is limited to 256 levels and a decoded list is limited to 1,000,000 elements.

## Errors

Both operations return `Result` values with the common `Error` family on failure; handle them with `try` or `catch`. Codes include `json-syntax` for invalid UTF-8, malformed input, or extra top-level values; `json-type` for a value with the wrong JSON type; `json-field` for unknown, duplicate, or missing required fields; `json-range` for integers or list sizes outside supported limits; and `json-depth` for nesting beyond the depth limit.

`error.detail(problem, "path")` returns an RFC 6901 JSON Pointer to the offending value, such as `/settings/retries`; `/` and `~` in a key are escaped as `~1` and `~0`. The root path is the empty string. Field errors also provide a `reason` such as `unknown`, `duplicate`, or `missing`; type errors provide `expected` and `actual`, and syntax errors provide an `offset`.

For example, an invalid supplied type reports the path rather than falling back:

```zhex
type Settings = record
    retries: Int
    verbose: Bool

let settings = json.decode("{\"retries\":\"many\"}", Settings(3, false)) catch problem
    print(error.code(problem))
    print(error.detail(problem, "path"))
    Settings(3, false)
print(settings.retries)
```

This prints `json-type`, `/retries`, then `3`.

Related guides: [records and named types](records.html), [lists and arrays](collections.html), [maps](maps.html), and [results and error handling](results.html).
