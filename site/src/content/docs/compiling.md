---
title: Compiling
description: Turn a checked Zhex program into a standalone WebAssembly module or a native Go binary with zhex compile.
section: Tools
order: 115
---

`zhex compile` turns a Zhex program into a standalone WebAssembly module or a native Go binary. The whole program is checked first, exactly as `zhex run` checks it — names, types, and word connections — and a program that fails the checker produces no artifact.

## Compile a program

```sh
zhex compile examples/hello.zhex
```

This writes `examples/hello.wasm` beside the source. `-o` chooses another output path, and `--target=` chooses the host layer the module talks to (`--target=native` instead leaves WebAssembly for the native target below):

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

## The native target

`--target=native` compiles the same checked program to a native binary through Go:

```sh
zhex compile examples/collatz.zhex --target=native
```

The output is a self-contained program directory — `examples/collatz/` beside the source, holding a readable `main.go`, a `go.mod`, and the `zhexrt` runtime sources — so the generated code can be read and reviewed like any Go program. Words become Go functions with concrete signatures, and a failure at runtime renders exactly the diagnostic `zhex run` prints, source line and caret included.

When the Go toolchain is installed, `zhex compile` builds the binary itself with `go build` (cross-compilation follows `GOOS`/`GOARCH`). Without Go, the sources and `go.mod` are still written and the compile prints the one instruction that finishes the job:

```text
go is not installed; the program source is in examples/collatz — build it with: (cd examples/collatz && go build)
```

Native binaries keep the interpreter's full signed 128-bit integers — there is no 64-bit limit and no fencing of wide values.

The native compiler covers a different subset than WebAssembly: it has console arguments and exits, `error.*` inspection, and word parameter and record field defaults, while it fences generic choices, typed `json.decode` and `xml.decode` (the `Json` tree words `json.parse` and `json.encode` compile), the temporal words, `file.free-space`, `error.set-reporter`, constructor partials, checked indexed writes, and callbacks that can fail — each with the same `not-available-in-build` diagnostic naming the construct.

Measured across the benchmark workloads, native binaries run at a 10.6× geometric-mean speedup over the interpreter, 3.9–70× per workload ([measurements](https://github.com/desireco/zhex/blob/main/docs/measurements.md)).

## The wasm-tools requirement

The WebAssembly targets need the free [wasm-tools](https://github.com/bytecodealliance/wasm-tools) assembler; it is the only extra tool they add, and running a compiled module needs nothing else in a browser. The native target needs no extra tool beyond Go itself. One-line installs:

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

Most of the [examples corpus](../examples.html) compiles to WebAssembly: words and recursion, flows, collections, text work, JSON trees, XML decode, buffers, files, and reading input. Some constructs are not compiled yet — dates and time zones beyond `duration.` words, console arguments and exits, `error.*` inspection other than `error.message` and `error.detail`, word parameter and record field defaults, generic choices, and a few checked collection forms.

WebAssembly programs use 64-bit integers by design — WebAssembly has no wider integer type. A literal beyond the signed 64-bit range refuses with the `not-available-in-build` diagnostic below, and computed arithmetic past the bound reports `integer overflow`, while `zhex run` and the native target compute the same program at the interpreter's full 128-bit width.

A program that uses one stops with the `not-available-in-build` diagnostic naming the construct:

```text
fence.zhex:1:9: timezone.load is not available in this build
    let t = timezone.load("UTC") try
            ^
hint: this construct is not compiled by zhex compile yet; run it with zhex run instead
```

Run such a program with `zhex run`, or rewrite it with constructs the build supports. The code is listed in the [error catalog](../errors.html#not-available-in-build).

Compiled programs are held to the interpreter: the repository's test suite compiles the examples corpus for WebAssembly and for the native target, and requires the same output, byte for byte, and the same exit status as `zhex run`.

## Where to go next

- [Getting started](getting-started.html) builds the interpreter and runs a first program.
- [Diagnostics and tracing](diagnostics.html) explains checker errors, JSON reports, and `--trace`.
