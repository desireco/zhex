# Contributing to Zhex

Thanks for helping! This guide is short on purpose.

## Reporting an issue

Just [open an issue](https://github.com/desireco/zhex/issues/new/choose). No need to search first or write formally. The most useful things to include:

- The smallest Zhex program that shows the problem
- The command you ran and its full output
- What you expected instead

Confusing error messages count as issues. So do docs that don't make sense.

## Suggesting a change to the language

The language is deliberately small, and every new word is a long-term commitment. Open an issue describing what you're trying to do before writing code — language ideas get discussed before any implementation.

## Sending a pull request

1. Small fixes (typos, docs, an example) can go straight to a PR.
2. For anything larger, comment on the issue first so nobody duplicates the work.
3. Make sure these pass:

   ```sh
   just test        # interpreter, examples, and website build
   go vet ./...
   ```

4. If you change how the language behaves, add or update tests in `internal/lang` — tests are the language definition.

No commit message format required. This repo uses `feat:`/`fix:`/`docs:` prefixes, but we can retitle when merging.

## Repo map

- `internal/lang/` — the interpreter (syntax, checker, eval) and its tests
- `cmd/zhex/` — the CLI
- `examples/` — validated example programs; the website runs them at build time
- `docs/` — language contract, guides, and plans
- `site/` — the website

## Setup

Install Go 1.23 or newer and [just](https://github.com/casey/just) (`just --list` shows all commands). Then:

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
go run ./cmd/zhex run examples/hello.zhex
```

The website build (`just site`, and the same build inside `just test`) compiles the examples' browser WebAssembly artifacts, so it also needs the free [wasm-tools](https://github.com/bytecodealliance/wasm-tools) assembler — the only extra tool. Install it with `cargo install wasm-tools`, `pacman -S wasm-tools`, `brew install wasm-tools`, or a [release binary](https://github.com/bytecodealliance/wasm-tools/releases); without it the site build fails right away with these hints. The [compiling guide](site/src/content/docs/compiling.md) covers targets and the requirement.

## License

By contributing, you agree your contributions are licensed under the [MIT License](LICENSE).
