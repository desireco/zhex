# Zhex language goals

Zhex aims to make small, understandable operations compose into useful, high-performance programs. Its syntax should make the program's logic easy for people to read, review, and change, including when an AI assistant generates the source.

Zhex is a Forth-inspired language. Source files use `.zhex`, and the planned command-line tool is `zhex`.

The intended application range includes command-line utilities, video games, web applications and servers, HTML and text processing, and database applications and systems. These are long-term goals. The first implementation is a small interpreter written in Go that tests the language's readability and semantics.

## Readability and simple syntax

Readers should understand ordinary code without mentally simulating stack permutations. Named inputs and local values are normal tools. Conditions, evaluation order, data transformations, and failure paths should be visible in the source.

Use a small, consistent grammar with clear indentation and few special cases. Prefer one obvious spelling for common operations. Familiar arithmetic and comparisons should remain easy to recognize. Stack composition is valuable where it clarifies a sequence of transformations; the language should not demand manual stack manipulation for routine application logic.

Judge syntax on complete examples: validation with several conditions, text processing, database queries, resource cleanup, and state updates. Attractive arithmetic examples alone do not establish readability.

## Small functions and explicit modules

A function, traditionally called a word in Forth, should express a focused operation. Small words compose through explicit arguments, return values, and readable flows. Their contracts should make inputs, outputs, and important effects understandable without inspecting every implementation.

Modules should have explicit imports, public interfaces, and qualified names. Applications should grow by combining reusable modules. Domain libraries should provide useful operations for files, structured data, databases, graphics, and web protocols without expanding the language's core grammar for each domain.

## Predictable behavior

Programs should be easy to reason about locally. Define evaluation order, lexical scope, numeric behavior, data ownership, mutation, and error propagation. Check types and composition before execution where practical, and report failures at their source locations.

Resource cleanup should be predictable on both normal completion and errors. Memory reclamation and external-resource lifetime are separate concerns. The language should make working with files and database connections understandable without requiring a general borrowing system in ordinary code.

## Human review of AI-generated code

AI generation should benefit from consistent syntax, explicit interfaces, small functions, and useful compiler feedback. The same properties must make the generated code easy for a person to inspect and correct.

Aim for stable formatting, discoverable APIs, actionable diagnostics, and runnable examples. Treat improved AI generation as a hypothesis to evaluate with representative tasks: check correctness, repair effort, and human comprehension rather than assuming fewer tokens means better code.

## Performance

High performance is a language goal, including execution time, allocation rate, memory use, startup time, and latency where the application needs it. Performance should come from a clear execution model, suitable data representations, efficient libraries, and optimization that preserves understandable semantics.

Measure text scanning and transformation, HTML workloads, function composition, collections, and database access. Game workloads also need predictable frame times; servers need useful throughput and latency measurements. Database measurements must distinguish language overhead from storage and query costs. Establish numerical targets when representative workloads and deployment targets exist.

The Go interpreter is an executable model of the language. It can reveal avoidable overhead and validate behavior, but its speed and garbage collector do not define the eventual compiled language's performance or memory policy. Native compilation through C remains a candidate, with other targets considered as their use cases become concrete.

## Application goals

| Application | Required direction | Representative demonstration |
| --- | --- | --- |
| Command-line utilities | Arguments, streams, files, structured data, useful errors, practical distribution | Scan files or transform text with bounded memory and clear exit behavior. |
| Video games | Efficient state updates, graphics/input/audio access, controllable allocations, predictable frame times | A small interactive 2D game with a measured update and rendering loop. |
| Web applications and servers | HTTP integration, routing, structured data, templates, deployment, and deliberate concurrency | A service that accepts input, queries data, and returns a correctly escaped HTML response. |
| HTML and text processing | Efficient strings and byte buffers, scanning, parsing, traversal, and transformation | Parse real HTML using a proper parser and extract or transform content; keep text and binary handling explicit. |
| Database applications | Database bindings, parameterized queries, transactions, typed results, and scoped connections | A small SQLite-backed catalog with predictable transaction and connection cleanup. |
| Database systems | Efficient buffers and indexes, explicit resource behavior, and eventual storage/concurrency facilities | Later, a small indexing or storage component with measured behavior and correctness checks. |

Web servers and browser applications have different host requirements. Browser execution is a future target decision; web application goals also include server-generated HTML. Database bindings are an early practical direction; writing a database engine is a later systems workload with additional durability and concurrency requirements.

## Foundation and influences

- **Forth:** small composable words and an immediate development loop.
- **Mirth:** checked composition and a compiler architecture that can target existing native toolchains.
- **Factor:** readable composition, local names, useful libraries, inspection, and resource handling.
- **8th:** structured managed values, modular application facilities, and database integration.

Use these systems as design references, as discussed in the [implementation survey](forth-implementation-research.md). Zhex's grammar and guarantees should follow its own readability and application goals; Forth compatibility is not a requirement.

## First implementation

The first interpreter will be written in Go. Console input and output are part of the first milestone, using interactive greeting, FizzBuzz, Fibonacci, factorial, and palindrome examples alongside checked composition. It now includes a REPL (`zhex repl`), and should next grow toward explicit modules and enough text/file support to build a useful command-line tool.

Keep parsing, semantic checking, execution, and host operations separate so later compilation can reuse the language definition and frontend. Prefer the smallest working representation; a second backend or a general virtual machine is not required to establish those boundaries.

Actors, zero-copy transfer, a custom reference-counted heap, graphics frameworks, HTTP frameworks, and a database engine are later work. The first interpreter should leave room for them through clear interfaces rather than implementing their infrastructure in advance.

## How we will judge the design

1. A reader can explain the result and evaluation order of representative examples without tracking stack shuffles.
2. Small words and explicit module interfaces compose into a complete useful application.
3. Invalid programs receive specific errors with source locations before side effects begin where statically detectable.
4. File execution and interactive execution agree on language semantics.
5. Resource cleanup and failure behavior are demonstrated in executable examples.
6. Performance claims name a workload, implementation, machine, and measurement method.

The immediate goal is the [first milestone](first-milestone.md): execute readable checked composition from a file. Earlier [design notes](design-notes.md) preserve candidate memory and concurrency ideas; those proposals remain open until specified and tested.
