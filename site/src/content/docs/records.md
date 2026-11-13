---
title: Records and named types
description: Declare nominal record types with named fields and construct values in field order.
section: Language
order: 25
---

A record gives a named type to a fixed set of typed fields. A record declaration is Zhex's first named-type representation; it is not another spelling for a type alias.

```zhex
type Customer = record
    name: String
    age: Int

word display-name(customer: Customer) -> String
    customer.name

let ada: Customer = Customer("Ada", 36)
print(display-name(ada))
```

The constructor takes field values positionally in declaration order, with exactly one value of the declared type for each field. The checker verifies the constructor arguments and each field read. Record fields are immutable after construction. Record types are nominal: two record declarations with the same fields still name distinct types. Type names cannot collide with a word or another type in the same module. You can use a declared record type in binding annotations, word signatures, and other field declarations; `let` and `var` infer the type as before. Recursive record declarations, including cycles through collections, are not supported.

An explicit annotation can be written on a binding as `let name: Type = expression` or `var name: Type = expression`. An annotation checks the value against the declared type; it does not convert the value.

Collections can be fields, and a record stores their shared reference. A `let` name cannot be rebound, but a collection referenced by that name can still be changed through its collection operations. See [lists and arrays](collections.html). Record fields cannot contain `Result` values, and concrete error families cannot be hidden in an `Error` field without explicit `error.wrap`.

An underscore-prefixed type is private to its module. An imported module's public word signatures and public record fields cannot expose a private type, including inside `List`, `Array`, or `Result` types. Public types are qualified at the import site; types declared in the module use their local name:

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
