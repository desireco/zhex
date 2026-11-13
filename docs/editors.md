# Zhex editor support

The repository includes syntax highlighting for `.zhex` files in Vim/Neovim, VS Code, and Zed. Themes choose the colors. The packages recognize current keywords, types, word definitions, calls, numbers, strings, escapes, and comments. They provide basic four-space indentation settings and line-comment support.

These are local packages. Extension-store publication, language-server completion, live compiler diagnostics, and formatting are future work. Clone the public repository to obtain the source.

## Vim and Neovim

From the repository root, install the Vim and Neovim packages with:

```sh
just vim-install
```

The installer links this checkout into `~/.vim/pack/zhex/start/zhex` and `~/.local/share/nvim/site/pack/zhex/start/zhex`. Neovim uses `$XDG_DATA_HOME` instead of `~/.local/share` when set. Keep the checkout at its current path. Repeating the command is safe; it refuses to replace a different existing plugin.

Restart Vim or Neovim after installation, then open a `.zhex` file. Alternatively, add the repository's `editors/vim` directory to your runtime path. Use an absolute path to your checkout:

```vim
set runtimepath^=/absolute/path/to/zhex/editors/vim
filetype plugin on
syntax enable
```

In a Neovim Lua configuration, the equivalent is:

```lua
vim.opt.runtimepath:prepend('/absolute/path/to/zhex/editors/vim')
vim.cmd('filetype plugin on')
vim.cmd('syntax enable')
```

Reopen a `.zhex` file. `:set filetype?` should report `zhex`. The plugin sets buffer-local spacing and comment options, and defines an undo command for those options when the filetype changes. It uses standard Vim syntax files, so no Tree-sitter installation is required for Vim or Neovim.

## VS Code

Build the local extension using a current Node.js LTS release and npm:

```sh
cd editors/vscode
npm ci
npm test
npm run package
```

The package is written to `dist/editors/zhex-0.0.1.vsix`.

In VS Code, run **Extensions: Install from VSIX** and select that file. Alternatively:

```sh
code --install-extension dist/editors/zhex-0.0.1.vsix
```

Open a `.zhex` file and confirm the language mode is **Zhex**. The extension adds TextMate highlighting, `#` comments, parentheses/quote pairing, and indentation rules. Its version and publisher identifier describe the local package; it has not been published to the Marketplace.

## Zed

The Zed extension uses the included Tree-sitter grammar. Prepare a dev-extension folder that refers to the grammar in your local Git checkout:

```sh
just zed-prepare
```

Run this from the repository root with Go 1.23 or newer and Git installed. Without `just`, run `go run ./tools/prepare-zed`. The command writes `dist/editors/zed` and prints its full path. In Zed, open **Extensions**, choose **Install Dev Extension**, and select that directory. Zed builds the grammar and recognizes `.zhex` files as **Zhex**. The helper changes only generated files under `dist/`; it does not modify editor settings.

Zed downloads its grammar build tools when needed. See [Zed's dev-extension instructions](https://zed.dev/docs/extensions/developing-extensions) for Rust and WASI SDK requirements. If installing from `editors/zed` directly, its pinned remote grammar fetches from the GitHub repository; the prepared local folder avoids that network fetch.

Zed also receives bracket matching and a word-definition outline. The editor grammar recognizes statements line by line for highlighting; the Go interpreter remains responsible for validating indentation, scope, and types.

## Verification and maintenance

The same [highlight fixture](../editors/testdata/highlighting.zhex) exercises Vim/Neovim and VS Code. It includes qualified and hyphenated names, keyword-like names such as `if-ready`, string-contained `#` characters, escapes, and subtraction. Tree-sitter has parse cases and highlight assertions, and is checked against all six interpreter examples.

Run the checks from the repository root:

```sh
vim -Nu NONE -n -es -S editors/vim/test/highlight.vim
nvim --headless -u NONE -n -S editors/vim/test/highlight.vim
npm --prefix editors/vscode test
```

With Tree-sitter CLI 0.26.9 installed:

```sh
cd editors/tree-sitter-zhex
tree-sitter test --config-path test/config.json
tree-sitter parse --grammar-path . --config-path test/config.json --quiet ../../examples/*.zhex ../testdata/highlighting.zhex
```

After changing the Tree-sitter grammar, run `tree-sitter generate --abi 14` and check in the generated `src/` files. Keep `queries/highlights.scm` and Zed's copy identical. Zed's manifest pins a commit containing the generated parser; update that revision when the grammar changes, then rerun the local preparation command.

The VS Code packaging tools pin a patched `minimatch` version through an npm override. Remove that override when `@vscode/vsce` directly uses a fixed release. Development dependencies are excluded from the VSIX.

The implementation follows the official [Vim/Neovim runtime](https://neovim.io/doc/user/syntax/), [VS Code TextMate](https://code.visualstudio.com/api/language-extensions/syntax-highlight-guide), and [Zed language extension](https://zed.dev/docs/extensions/languages) mechanisms.
