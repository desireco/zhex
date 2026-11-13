# Diagnostic catalog

Every Zhex diagnostic carries one of these stable codes. The first line of a
diagnostic is always `file:line:column: message`; the source excerpt, caret,
and `hint:` line are additive detail. Machine readers use
`zhex run <file> --report=json`.

## source-utf8
The file is not valid UTF-8. Re-save it as UTF-8; Zhex source has no other encoding.

## tab-character
A tab appeared outside a string. Indentation is four spaces per level; replace tabs with spaces.

## indentation
A line's indent is not a multiple of four spaces, or it opens more than one level at once. Each block indents exactly four spaces from its parent.

## unterminated-string
A string literal has no closing quote before the end of the line. Close it on the same line; strings do not continue across newlines.

## string-escape
A string contains an escape Zhex does not support, or invalid UTF-8. Strings are double-quoted and use Go-style escapes like `\n` and `\"`.

## unexpected-character
A character cannot start any token. Check for stray punctuation or a non-ASCII symbol; names are ASCII letters, digits, underscores, and hyphenated segments.

## expected-token
The parser expected a specific token and found something else. The message names both; fix the line to match the expected grammar.

## expected-name
A keyword appears where a name is required. `word`, `let`, `var`, `if`, and the other keywords are reserved; choose a name that is not one of them.

## unknown-type
A type name is not one of `Int`, `Bool`, `String`, or `Unit`. Check the spelling; there are no user-defined types yet.

## expected-indent
A construct needs an indented block on the following lines. Indent the body four spaces; parentheses do not continue a line.

## empty-block
A block header is followed by nothing. Every block needs at least one statement.

## nesting-limit
Expressions or blocks nest past the parser's limit. Split the expression into named parts with `let`.

## integer-literal-range
An integer literal is outside the signed 64-bit range. Keep literals within ±9223372036854775807; compute larger values at runtime.

## expected-expression
A token cannot start an expression where one is required.

## flow-step
A `flow` block needs at least one step word. Write one word per line; the first receives the seed value.

## unknown-name
No binding or word has this name. The hint suggests the closest known one; if the name contains a hyphen, that hyphen is part of the name — subtraction is written `a - b` with spaces.

## duplicate-name
This name is already defined in the same scope. Choose a fresh name; shadowing is not allowed.

## duplicate-word
A word with this name already exists, or it collides with a host word. Rename one of them.

## word-result
The word's final statement produces a different type than its declared result. Make the last statement's type match the signature, or fix the signature.

## immutable-name
A name defined with `let` cannot be reassigned. Declare it with `var` to allow updates.

## assignment-type
The new value's type differs from the variable's type. There are no implicit conversions; convert explicitly (for example `int.to-string`).
When the checker can trace the value to the word that produced it, the message says where it comes from.

## condition-type
An `if` condition must be `Bool`. Comparisons return `Bool`; integers and strings do not.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## branch-type
The branches of an `if` end in different types. Make every branch produce the same type.

## range-type
The bounds of `range(start, end)` must both be `Int`.

## try-input
Postfix `try` needs a `Result` value to unwrap. It follows a `read-line` or `text.to-int` call — the only operations that return a `Result`.

## unary-type
The unary operator (`-`, `not`) received the wrong type. `-` needs `Int`; `not` needs `Bool`.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## operand-types
A binary operator got two different operand types. If one side is a `Result`, add postfix `try` to unwrap it.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## bool-operand
`and`/`or` need `Bool` on both sides. Comparisons produce `Bool`; integers do not.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## compare-type
Equality comparison needs matching `Int`, `Bool`, or `String` values. If one side is a `Result`, add postfix `try` to unwrap it.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## ordering-type
Ordering comparison (`< <= > >=`) needs `Int` operands.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## addition-type
`+` adds two `Int`s or concatenates two `String`s — never one of each. If you meant concatenation, convert with `int.to-string` first.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## arithmetic-type
`- * / %` need `Int` operands.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## argument-count
A word call passed the wrong number of arguments. The message states how many the word takes.

## argument-type
An argument's type does not match the parameter. There are no implicit conversions; convert at the call site.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## unknown-word
No word with this name exists. The hint suggests the closest known word; check the spelling or define it.

## print-type
`print` accepts one `Int`, `String`, or `Bool` value. If it received a `Result`, add postfix `try` to unwrap it.
When the checker can trace the value to the word that produced it, the message says where it comes from.

## range-context
`range` exists only inside `for n in range(start, end)`. It cannot be called elsewhere.

## integer-overflow
An arithmetic result does not fit in a signed 64-bit integer. Overflow is an error in Zhex, not a wraparound; keep intermediate values in range.

## division-zero
Division or remainder by zero. Guard the divisor with a condition first.

## depth-limit
Word recursion passed the runtime depth limit of 256. Rewrite the recursion as a loop with `var` and `for`; loops have no depth limit.

## unsupported-code
An internal invariant failed: the checker approved something the evaluator cannot run. This is a bug in Zhex, not in your program — please report the program that triggered it.

## console-output
Writing to the output stream failed. Check that the output is writable (closed pipes fail this way).

## console-input
Reading a line from the input stream failed, usually because the input ended. The failure propagates through `try` and exits the program; check that input is available.

## console-utf8
The line read from the input stream is not valid UTF-8. Zhex reads UTF-8 text; supply the input as UTF-8.

## text-to-int
The text given to `text.to-int` is not a signed 64-bit decimal integer. It accepts only digits, with an optional sign; surrounding whitespace is ignored.
