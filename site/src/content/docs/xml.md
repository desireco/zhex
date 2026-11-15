---
title: XML
description: Decode typed XML documents with explicit defaults, attribute fields, and repeated-element lists.
section: Standard library
order: 112
---

The `xml` operation converts XML text into checked Zhex values. Decoding requires a value of the target type to supply defaults for missing record fields:

```zhex
type Endpoint = record
    host: String
    port: Int

type Service = record
    name: String
    retries: Int
    dry-run: Bool
    endpoints: List<Endpoint>
    owner: Option<String>

let service = xml.decode(
    "<service retries=\"3\" dry-run=\"true\"><name>ingest</name><endpoints><host>db</host><port>5432</port></endpoints></service>",
    Service("backup", 1, false, [], Option.None)
) try
print service.name
print(list.length(service.endpoints))
```

Output:

```text
ingest
1
```

`xml.decode(text, defaults)` returns `Result<T, Error>`, with `T` inferred from the defaults value. The mapping is schema-directed, like `json.decode`: attributes map onto same-named record fields verbatim alongside child-element fields, repeated same-named child elements collect into a `List<T>` field in document order, and an absent element or attribute uses its corresponding default.

| Operation | Signature | Behavior |
| --- | --- | --- |
| `xml.decode(text, defaults)` | `(String, T) -> Result<T, Error>` | Decode XML using the defaults value to determine `T` and fill absent record fields. |

## Mapping rules

- **Attributes are fields.** Each `attr="value"` on an element feeds the same-named record field. An attribute name colliding with a child element name in the same element fails the decode — strict, mirroring `json.decode`'s duplicate-key rule.
- **Repeated elements are lists.** Sibling elements sharing a name feed a `List<T>` field in document order; two or more elements for a non-list field fail as duplicates.
- **Absent means default.** An absent element or attribute uses the field's default from the defaults value. An `Option<T>` field with an `Option.None` default decodes as `Option.None` when absent and `Option.Some(...)` when present — XML has no null marker.
- **Text-only elements are scalars.** An element with only text decodes as `String` (exact text), `Int` (integer-form signed 64-bit, surrounding whitespace trimmed), or `Bool` (exactly `true` or `false`).
- **Lists and maps nest by children.** For a `List<T>` target the element's children are the items; for a `Map<String, T>` each child element's name is the key and its content the value, in document order.
- **Whitespace between elements is ignored**; non-whitespace text next to child elements is mixed content and fails.

The decoder checks every supplied value. It rejects unknown elements and attributes, missing required fields with no default, wrong types, and duplicate or colliding names; it never substitutes a default for an invalid supplied value. Mutable defaults are freshly copied for each decoded result, and present lists replace their defaults instead of merging — the same cloning rules as `json.decode`.

## Supported values

XML supports `Int`, `Bool`, `String`, records, `List<T>`, `Map<String, T>`, and `Option<T>`, including nested combinations of those types. Direct `Option<Option<T>>` is rejected, as in JSON. Attributes can supply only scalar or `Option<T>` fields; structure comes from child elements. Lists and maps of records do not provide a per-item default record: each supplied record item must include every field.

Namespaces are not supported in v1: namespace-prefixed names and a default `xmlns` (which qualifies its elements) are rejected rather than silently flattened. DTD and entity declarations are rejected at parse; the five predefined entities (`&amp;` `&lt;` `&gt;` `&quot;` `&apos;`) and numeric character references decode, and nothing is ever fetched from the network. Nesting is limited to 256 levels and a document to 1,000,000 elements.

Unsupported types — `Unit`, Results, buffers, files, arrays, choices other than `Option`, maps with non-string keys — are rejected by the checker before execution. There is no XML encoding in v1.

## Errors

Failures use the common `Error` family with stable codes: `xml-syntax` (malformed XML, DTD or entity declarations, undefined entities, multiple root elements), `xml-type` (wrong type, mixed content, an attribute where structure is required), `xml-field` (unknown, duplicate, missing, or colliding names, and namespace-qualified names), `xml-range` (integer overflow, document or list over the element limit), and `xml-depth` (nesting over 256).

`error.detail(problem, "path")` carries the element path — `/server/endpoints/0/host`, with the root as the empty string and list items by index. `reason` is one of `unknown`, `duplicate`, `missing`, `collision`, `prefixed`, `element-limit`, and `list-limit`; type failures carry `expected` and `actual`; syntax failures carry `offset`, `line`, and `column`.

```zhex
type Service = record
    name: String
    retries: Int
let result = xml.decode("<service retries=\"soon\"/>", Service("backup", 1)) catch problem
    print(error.code(problem))
    print(error.detail(problem, "path"))
    Service("backup", 1)
```

Output:

```text
xml-type
/retries
```
