---
title: Compiling to WebAssembly
description: Turn a checked Zhex program into a standalone WebAssembly module with zhex compile.
section: Tools
order: 115
---

`zhex compile` turns a Zhex program into a standalone WebAssembly module. The whole program is checked first, exactly as `zhex run` checks it — names, types, and word connections — and a program that fails the checker produces no artifact.

## Compile a program

```sh
zhex compile examples/hello.zhex
```

This writes `examples/hello.wasm` beside the source. `-o` chooses another output path, and `--target=` chooses the host layer the module talks to:

```sh
zhex compile examples/hello.zhex -o hello.wasm --target=wasi
```

## Targets

- `--target=browser` (the default) produces a module for a web page or JavaScript host. The module imports a small set of functions — printing, reading input, and file access — that the surrounding JavaScript provides. The repository's test harness [`internal/wasm/testdata/run.mjs`](https://github.com/desireco/zhex/blob/main/internal/wasm/testdata/run.mjs) is a complete, working host in a few hundred lines of JavaScript.
- `--target=wasi` produces a standalone module that runs under a WebAssembly runner such as [wasmtime](https://wasmtime.dev/), with file access limited to the directories you preopen. The [collatz example](../examples.html#collatz) compiles and prints the same steps as the interpreter:

```sh
zhex compile examples/collatz.zhex --target=wasi
wasmtime run --dir .::/ examples/collatz.wasm
```

A program that reads input takes it on standard input, so pipe the name in: `printf 'Ada\n' | wasmtime run --dir .::/ examples/hello.wasm`.

## The wasm-tools requirement

Compiling needs the free [wasm-tools](https://github.com/bytecodealliance/wasm-tools) assembler; it is the only extra tool, and running a compiled module needs nothing else in a browser. One-line installs:

```sh
cargo install wasm-tools        # rustup/cargo, any platform
pacman -S wasm-tools            # Arch Linux
brew install wasm-tools         # macOS
```

A prebuilt release binary is also available on the [wasm-tools releases page](https://github.com/bytecodealliance/wasm-tools/releases). Without it, `zhex compile` stops with a coded diagnostic instead of a raw error, and no artifact is written:

```text
wasm-tools is not installed; zhex compile needs it to assemble the WebAssembly module (code: missing-tool)
hint: install it — cargo install wasm-tools (rustup), pacman -S wasm-tools (Arch), or brew install wasm-tools (macOS)
```

## What compiles today

Most of the [examples corpus](../examples.html) compiles: words and recursion, flows, collections, text work, JSON trees, XML decode, buffers, files, and reading input. Some constructs are not compiled yet — dates and time zones beyond `duration.` words, console arguments and exits, `error.*` inspection other than `error.message` and `error.detail`, word parameter and record field defaults, generic choices, and a few checked collection forms.

A program that uses one stops with the `not-available-in-build` diagnostic naming the construct:

```text
fence.zhex:1:9: timezone.load is not available in this build
    let t = timezone.load("UTC") try
            ^
hint: this construct is not compiled by zhex compile yet; run it with zhex run instead
```

Run such a program with `zhex run`, or rewrite it with constructs the build supports. The code is listed in the [error catalog](../errors.html#not-available-in-build).

Compiled programs are held to the interpreter: the repository's test suite compiles the examples corpus and requires the same output, byte for byte, and the same exit status as `zhex run`.

## Where to go next

- [Getting started](getting-started.html) builds the interpreter and runs a first program.
- [Diagnostics and tracing](diagnostics.html) explains checker errors, JSON reports, and `--trace`.
