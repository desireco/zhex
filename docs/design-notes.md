# Initial language design notes

Captured from **Make Forth Great Again**, November 5–7, 2026. These notes preserve the direction of exploration; recommendations remain subject to design and measurement.

Current direction: Go has been selected for the first interpreter. See the [language goals](language-goals.md) for the current scope. The implementation-language question below records the earlier discussion; memory, concurrency, and backend proposals remain open.

## Goals

- Keep Forth-style stack composition while making source code easier to read.
- Explore Python-style indentation and Lisp-like nested expressions.
- Reuse existing compilation and platform infrastructure.
- Support practical CLI tools, terminal UIs, modules, and SQLite integration.
- Keep resource and memory handling understandable, safe, and efficient.

## Syntax candidates

An indented word definition could preserve ordinary stack execution:

```text
word square
    dup
    multiply
```

Nested arguments could evaluate from top to bottom before their parent operation:

```text
multiply
    add
        2
        3
    4
```

This would lower to `2 3 add 4 multiply`. Explicit `flow` blocks were also explored for pipelines. Local names should be available where stack rearrangement becomes hard to follow. Branches should have compatible stack shapes and types. Exact syntax and typing rules remain open.

## Compilation targets

- **C:** leading candidate for native executables and a path to WebAssembly through existing toolchains. Runtime memory, actors, and host integration still require design.
- **BEAM:** attractive when actors, scheduling, messaging, and existing runtime services dominate. It brings BEAM's memory and execution model.
- **Direct WebAssembly:** plausible for a typed stack language; requires data layout, runtime support, and host interfaces.
- **Rust and JavaScript:** considered as alternative targets.

Keep parsing and semantic checking separate from code generation. Select one initial backend rather than implementing all candidates at once. No final target was selected.

## Application structure

Use modules with explicit public interfaces and qualified names. Separate modules from packages and runtime resource instances. Parse CLI options into a record passed to the words that need it. Use records, arrays, strings, and optional local names for structured application data.

## External resources

The latest proposed simplification was scoped cleanup with checked handles:

```text
"files.db" sqlite.open try

using db
    db "." catalog.files-in try
```

The scope owns cleanup. Handles may be copied, but copying does not create another cleanup obligation. Operations validate a resource-table slot, generation, and resource type; use after closure returns an error. Resources initially belong to one task or actor. Asynchronous operations must complete or finish cancellation before cleanup. Query results should generally be owned values independent of the connection.

This intentionally accepts runtime checks rather than requiring a general user-visible borrowing system.

## Data memory

The proposed starting point was value semantics with compiler-managed reference counting:

- Scalars and small records can live inline.
- Strings may share immutable reference-counted storage.
- Arrays use contiguous buffers and copy-on-write updates.
- Exclusively owned buffers may be updated in place.
- Compiler analysis may remove redundant retains/releases and reuse storage.

Reference counting is automatic reclamation, although it avoids a tracing collector. Cycles require restrictions or another explicit policy. Arbitrary shared mutable graphs were not part of the proposed first version. External resource lifetimes remain separate from data lifetimes.

## Actors and large arrays

The final discussion focused on a single ordinary `send` whose receiver gets a value the sender cannot subsequently change:

- Transfer an exclusively owned buffer when safe.
- Share immutable buffers across actors with thread-safe reference counting.
- Copy before updating shared storage.

These are potential zero-copy paths within a shared-memory runtime. Other processes, isolated Wasm memories, or remote hosts may require serialization or copying. Nested values must also be safe to send. Mailbox backpressure, scheduling costs, counter contention, and a correct exclusivity protocol still need design.

## Open decisions

1. Initial target and compiler implementation language.
2. Exact grammar, stack-effect notation, types, and numeric behavior.
3. Minimal standard library and module/package rules.
4. Cycle policy and closure capture semantics.
5. Actor scheduling, sendability, cancellation, and backpressure.
6. Error propagation and precise resource-scope rules.

The next useful milestone is an executable arithmetic example with checked stack effects, before committing to a full runtime.
