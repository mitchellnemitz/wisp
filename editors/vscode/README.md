# wisp for VSCode

One extension providing both halves of wisp editor support:

- **Syntax highlighting** via a TextMate grammar plus comment/bracket/auto-close
  behavior and `.wisp` association. This works as soon as the extension is
  installed, with nothing else to set up.
- **Language server** features (diagnostics, formatting, document symbols,
  hover, completion, go-to-definition, find-references, rename, signature help,
  and quick-fix code actions) by launching `wisp-lsp` and connecting to it over
  stdio. This is optional: if the server is missing or disabled, highlighting
  still works.

## Build and install the language server

The extension launches `wisp-lsp` but does not ship it. Build it from this
repository and put it on your PATH:

```sh
# from the repository root
go build -o wisp-lsp ./cmd/wisp-lsp
mv wisp-lsp /usr/local/bin/   # or anywhere on your PATH
# or: go install github.com/mitchellnemitz/wisp/cmd/wisp-lsp@latest
```

If you keep it elsewhere, set `wisp.lsp.path` to its absolute path. To use only
highlighting, set `wisp.lsp.enable` to `false`.

## Build and install the extension

This is a plain-JavaScript extension with one runtime dependency
(`vscode-languageclient`) and no compile step. Package it into a `.vsix` and
install that:

```sh
cd editors/vscode
npm install
npx --yes @vscode/vsce package
code --install-extension wisp-0.1.0.vsix
```

Install the packaged `.vsix`, not the raw folder: `code --install-extension
<folder>` is not supported.

For quick local iteration you can instead symlink the folder into your
extensions directory after running `npm install`, so `node_modules` is present:

```sh
npm install
ln -s "$PWD" ~/.vscode/extensions/wisp
```

Then reload VSCode and open a `.wisp` file.

## Settings

- `wisp.lsp.enable` (boolean, default `true`): run the language server.
- `wisp.lsp.path` (string, default `"wisp-lsp"`): the server executable, on
  PATH or an absolute path.
