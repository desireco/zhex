---
title: Maps
description: Store values by typed keys, look them up with Option, and traverse entries in insertion order.
section: Language
order: 36
---

A `Map<K, V>` stores one value for each key and grows as entries are added; there is no fixed entry-count limit. A nonempty map literal infers its key and value types; entries must use one exact key type and one exact value type:

```zhex
let scores = {"Ada": 10, "Grace": 9}
scores["Ada"] = 11
print(map.length(scores))
```

Keys in map literals are constant `String`, `Int`, or `Bool` values; key variables are not allowed in the literal. Negative integer keys are valid. Duplicate keys with the same type and value are rejected, including semantically equal spellings such as `0` and `-0`. Map literal values evaluate once, from left to right. An empty map needs an expected type:

```zhex
let empty: Map<String, Int> = {}
```

## Look up and update

Using `scores` from the first example, lookup returns the built-in `Option<V>`: a present key gives `Option.Some(value)` and a missing key gives `Option.None`. Match both outcomes. `Option.Some(value)` infers its type from the payload; a standalone `Option.None` needs an expected `Option<T>` type. `Option` is reserved for built-in lookup values.

```zhex
match scores["Ada"]
    case Option.Some(score)
        print(score)
    case Option.None
        print("No score")
```

Bracket assignment sets a value and returns `Unit`; it does not produce a `Result`. `map.remove(map, key)` removes a key and returns `Unit`; removing an absent key does nothing. `map.contains(map, key)` returns `Bool`, and `map.length(map)` returns the number of entries.

`map.put(map, key, value)` inserts or replaces an entry and returns the same map. `map.into(source, target)` copies entries from one map into another and returns the target. Both preserve insertion order: replacing an existing key keeps its position, and new keys append in source order. Removing a key and inserting it again puts it at the end. `map.into` applies its changes atomically; using the same map as source and target is a no-op unless that map is being traversed.

| Operation | Result | Behavior |
| --- | --- | --- |
| `map.length(map)` | `Int` | Number of entries. |
| `map.contains(map, key)` | `Bool` | Whether the key is present. |
| `map[key]` | `Option<V>` | Value wrapped in `Option.Some`, or `Option.None` when absent. |
| `map[key] = value` | `Unit` | Insert or replace a value. |
| `map.remove(map, key)` | `Unit` | Remove the key; absent keys are a no-op. |
| `map.put(map, key, value)` | `Map<K, V>` | Insert or replace and return the same map. |
| `map.into(source, target)` | `Map<K, V>` | Merge entries atomically and return the target. |

Maps cannot be printed or compared directly. Their values cannot contain `Result`; a caught concrete-family error also cannot be stored as an `Error` value without first converting it with `error.wrap`. See [results and error handling](results.html) for the error-family rule.

## Compose map operations in a flow

Using `scores` from the first example, a flow step can be a call with extra arguments. The preceding value supplies the call's first argument, and the listed arguments follow it once in left-to-right order:

```zhex
flow scores
    map.put("Nancy", 23)
    map.length
    print
```

Here `scores` is passed as the first argument to `map.put`; the step inserts `"Nancy"` and `23`, then passes the same map to `map.length`.

The next example passes a new literal map into `map.into`, with `scores` as its target:

```zhex
flow {"Lin": 12}
    map.into(scores)
    map.length
    print
```

The literal map becomes the first argument to `map.into`, while `scores` is its explicit second argument. The flow receives the target map back from `map.into` and prints its new length. Unary flow steps such as `map.length` keep their existing form; `try` remains postfix on any fallible step or argument.

## Iterate in insertion order

Using `scores` from the first example, `for value in map` visits values and `for key, value in map` visits both parts of each entry, with keys and values bound immutably for that iteration:

```zhex
for name, score in scores
    print(name + ": " + int.to-string(score))
```

Maps are shared mutable values. Assigning or passing one to another name refers to the same storage. Traversal uses insertion order, so the first inserted key is visited first and replacements keep their position. Mutating the same map through any alias while it is being traversed raises the `collection-iterating` runtime programmer diagnostic before the write, including an otherwise no-op removal. This is not a `Result` and cannot be handled with `catch`. Mutating a separate map is allowed. Map iteration does not impose a fixed element limit.

Related guides: [choices and matching](choices.html) for handling `Option`, [lists and arrays](collections.html), and [words and flows](words-and-flows.html).
