---
title: Records and named types
description: Declare nominal record types with typed fields, defaults, and positional or named construction.
section: Language
order: 25
---

A record gives a named type to a fixed set of typed fields. It is one of Zhex's named-type forms; a record declaration is not another spelling for a type alias.

```zhex
type Customer = record
    name: String
    age: Int

word display-name(customer: Customer) -> String
    customer.name

let ada: Customer = Customer("Ada", 36)
let ada-by-name = Customer(age: 36, name: "Ada")
print(display-name(ada))
print(display-name(ada-by-name))
```

Construct a record either positionally in declaration order or with field names. Named fields can be supplied in any order:

```zhex
type Settings = record
    retries: Int = 3
    verbose: Bool = false

let defaults = Settings()
let verbose = Settings(verbose: true)
print(defaults.retries)
print(verbose.retries)
print(verbose.verbose)
```

This prints `3`, `3`, and `true`. Required fields come first; trailing defaulted fields may be omitted. A constructor call must use all positional arguments or all named arguments; the forms cannot be mixed. This named-argument syntax applies only to record constructors: word calls, built-ins, and choice alternatives remain positional. A named record constructor can be a flow seed, as in `flow Settings(verbose: true)`; flow steps still receive the current value as their positional first argument and do not accept named arguments. The checker verifies constructor arguments and field reads. Record fields are immutable after construction. Record types are nominal: two record declarations with the same fields still name distinct types. Type names cannot collide with a word or another type in the same module. You can use a declared record type in binding annotations, word signatures, and other field declarations; `let` and `var` infer the type as before. Recursive record declarations, including cycles through collections, are not supported.

An explicit annotation can be written on a binding as `let name: Type = expression` or `var name: Type = expression`. An annotation checks the value against the declared type; it does not convert the value.

## Field defaults

Give trailing fields a default to omit them from a constructor call:

```zhex
type Settings = record
    retries: Int = 3
    verbose: Bool = false

let defaults = Settings()
let positional = Settings(5)
let named = Settings(verbose: true)
print(defaults.retries)
print(positional.retries)
print(named.retries)
print(named.verbose)
```

Required fields must precede defaulted fields. Named fields may be listed in any order, but each field can be supplied only once. Explicit arguments evaluate once in source order; omitted defaults then evaluate in field declaration order, in the type's defining module, each time a record is constructed. Defaults cannot refer to caller bindings or other fields and cannot propagate a Result. Fresh list or map expressions create independent collections; explicitly supplied collections keep their shared identity.

## Type aliases

An alias gives an existing type another name without creating a new type or constructor:

```zhex
type Names = alias List<String>
let names: Names = ["Ada"]
print(list.length(names))
```

Aliases work in annotations and signatures, can refer forward to other types, and follow module privacy rules. Cycles and generic alias declarations are rejected. An alias cannot hide a Result obligation.

Collections can be fields, and a record stores their shared reference. A `let` name cannot be rebound, but a collection referenced by that name can still be changed through its collection operations. See [lists and arrays](collections.html) and [maps](maps.html). Records and supported nested collection values can be encoded as JSON; see [JSON](json.html). Record fields cannot contain `Result` values, and concrete error families cannot be hidden in an `Error` field without explicit `error.wrap`.

An underscore-prefixed type is private to its module. An imported module's public word signatures and public record fields cannot expose a private type, including inside `List`, `Array`, `Map`, or `Result` types. Public types are qualified at the import site; types declared in the module use their local name:

`people.zhex`:

```zhex
type Customer = record
    name: String

word make(name: String) -> Customer
    Customer(name)
```

`main.zhex` in the same directory:

```zhex
use people

let ada: people.Customer = people.Customer("Ada")
let grace = people.make("Grace")
print(ada.name)
print(grace.name)
```

Run `zhex run main.zhex` from that directory to print `Ada` and `Grace`.

Records are values with identity by their nominal type; direct comparison and `print` formatting are not defined. Read fields or build a specific string when displaying a record.

Related guides: [values and bindings](values.html), [words and flows](words-and-flows.html), and [lists and arrays](collections.html).
