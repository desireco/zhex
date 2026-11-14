---
title: Syntax
description: Learn Zhex source layout, indentation, line endings, names, comments, strings, and expression syntax.
section: Language
order: 10
---

Zhex source is UTF-8 text. Statements end at a newline, and indentation marks blocks. The parser checks the whole file before the interpreter runs it.

## Indentation and lines

Use four spaces for each block level. Tabs outside strings and comments are rejected. Parentheses do not continue an expression onto another line.

```zhex
let ready = true
if ready
    print("Starting")
else
    print("Waiting")
```

Blank lines are ignored. A `#` starts a comment through the end of that line:

```zhex
# Read a line; try reports an input failure at runtime.
let line = read-line("Count: ") try
```

The parser limits syntax nesting and expression-tree depth to 256 so excessively deep source becomes a language diagnostic.

## Names

Names start with an ASCII letter or underscore and can contain ASCII letters, digits, and underscores. Hyphen-separated segments are allowed, so `is-palindrome` is one name.

That means spacing can change how a line is read: `a-b` is a name, while `a - b` subtracts. Put spaces around subtraction between names.

A dot qualifies an operation by its owner, as in `text.reverse` or `math.gcd`. The dot is not general punctuation for other names. A leading underscore on a module word makes that word private to its module; see [modules](modules.html).

Keywords such as `word`, `let`, `if`, `for`, `flow`, `try`, `and`, `or`, and `not` cannot be used as binding or word names. `use` is also reserved and only allowed at file top level.

## Strings

Strings use double quotes and Go-style escapes, including `\n`, `\t`, `\\`, and `\"`. A string must decode to valid UTF-8.

```zhex
let message = "first line\nsecond line"
print(message)
```

Zhex does not join arbitrary adjacent source lines into one expression. The block forms described above continue only through their indented body; keep ordinary expressions on one line.

## Expression shape

Expressions include literals, names, calls, unary operators, and binary operators. `if`, `match`, and `flow` are block forms; each can be used as the entire right-hand side of a `let`, `var`, or assignment, with its final value supplying the result. They are not general nested expressions or call arguments. Calls use parentheses: `greet("Ada")`. `try` is postfix: `read-line("Name: ") try`.

Here is the operator precedence from lowest to highest:

| Level | Operators |
| --- | --- |
| 1 | `or` |
| 2 | `and` |
| 3 | `==`, `!=` |
| 4 | `<`, `<=`, `>`, `>=` |
| 5 | `+`, `-` |
| 6 | `*`, `/`, `%` |
| 7 | unary `-`, `not` |
| 8 | calls, postfix `try` |

Binary operators associate left. Parentheses can make grouping explicit. `and` and `or` short-circuit. Arithmetic and type rules are in [values and operators](values.html); call and flow rules are in [words and flows](words-and-flows.html).
