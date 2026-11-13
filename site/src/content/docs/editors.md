---
title: Editor support
description: Install local Zhex syntax support for Vim, Neovim, VS Code, or Zed.
section: Tools
order: 130
---

The repository includes local syntax support for Vim/Neovim, VS Code, and Zed. These packages provide syntax highlighting and editor-specific indentation or bracket settings. They do not provide language-server completion, live compiler diagnostics, or formatting.

Clone the repository to get the editor package sources, then follow the matching install steps below.

## Vim and Neovim

From the repository root, link the package into Vim and Neovim's local plugin directories:

```sh
just vim-install
```

Restart the editor and open a `.zhex` file. The install script refuses to replace an existing plugin directory that points elsewhere. To use the checkout directly, add its `editors/vim` directory to your runtime path and enable filetype plugins and syntax highlighting.

## VS Code

Build the local VSIX package with Node.js and npm:

```sh
cd editors/vscode
npm ci
npm run package
```

In VS Code, run **Extensions: Install from VSIX** and select `dist/editors/zhex-0.0.1.vsix` (relative to the repository root). Open a `.zhex` file; its language mode should be **Zhex**.

## Zed

Prepare a dev-extension directory from the repository root:

```sh
just zed-prepare
```

The command writes `dist/editors/zed` and prints its full path. In Zed, open **Extensions**, choose **Install Dev Extension**, and select that directory. Zed may download its grammar build tools when needed; its development extension build has Rust and WASI SDK requirements.

The Zed package includes a word-definition outline. The Tree-sitter grammar supports editor parsing and highlighting; the Zhex interpreter remains responsible for checking indentation, scope, names, and types.
