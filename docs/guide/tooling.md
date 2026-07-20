# Editor tooling

wisp ships a language server and editor integrations for VSCode and Vim.
Syntax highlighting works on its own. The language server is optional and adds
diagnostics, formatting, document symbols, hover, and completion.

## The language server: wisp-lsp

`wisp-lsp` is a separate binary that speaks the Language Server Protocol over
stdio. It reuses the compiler's own lexer, parser, checker, and formatter, so
it never drifts from the language.

Build it the same way as the compiler:

```sh
go build -o wisp-lsp ./cmd/wisp-lsp
# or for a static binary:
CGO_ENABLED=0 go build -o wisp-lsp ./cmd/wisp-lsp
```

Put it on your PATH. Editors launch it with no arguments.

It implements:

- Diagnostics, pushed on open and change. A lex or parse error yields one
  diagnostic; otherwise the checker's errors and warnings are reported.
- Formatting, returning a single whole-document edit, or no edit on a parse
  error.
- Document symbols for functions and structs.
- Hover, reporting the kind and type or signature of the symbol under the
  cursor. This includes namespaced core-module members (`string.trim`,
  `array.map`, ...), which show the signature when one is statically known,
  and the namespace qualifier itself (`string` in `string.trim`), which
  reports the module sense rather than the type sense.
- Completion of keywords, types, builtins, reserved constants, and the
  functions, structs, and variables declared in the document. Typing a core
  namespace followed by `.` (a registered completion trigger character)
  offers that namespace's members.
- Go-to-definition for variables and functions.
- Find-references and rename for variables and functions, scope-aware so two
  variables with the same name in different scopes are not confused.
- Signature help while typing a call to one of your functions.
- A quick-fix code action that applies a "did you mean" suggestion.

## VSCode

The extension under `editors/vscode/` is one package that provides both
highlighting and the language server. See its
[README](../../editors/vscode/README.md) for the full install steps. In short:

1. Build `wisp-lsp` and put it on your PATH.
2. From `editors/vscode/`, run `npm install` and then `npx @vscode/vsce
   package`, and install the resulting `.vsix` with `code --install-extension`.

Two settings control the server:

- `wisp.lsp.enable` (default true): run the language server.
- `wisp.lsp.path` (default `wisp-lsp`): the server executable, found on PATH or
  given as an absolute path.

If the server is missing or disabled, highlighting still works.

## Vim and Neovim

The files under `editors/vim/` provide highlighting and filetype detection.
Put `syntax/wisp.vim` and `ftdetect/wisp.vim` on your runtimepath, or point a
plugin manager at the `editors/vim` directory. See its
[README](../../editors/vim/README.md) for wiring `wisp-lsp` through the Neovim
built-in LSP, nvim-lspconfig, or coc.nvim.

## Keeping highlighting in sync

The keyword, type, builtin, and constant sets in both editor grammars are
checked against the compiler's own tables by a test in `internal/editors`. A
name added to the language cannot silently fall out of the highlighters. This
also covers the namespace-qualifier surface: a call like `string.trim(x)`
highlights `string` as a namespace and `trim` as a builtin in both grammars,
and the set of core namespaces (`types.CoreNamespaces()`) is drift-guarded the
same way, so a namespace added to the compiler's core-module catalog cannot
silently fall out of either highlighter.
