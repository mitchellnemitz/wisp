# wisp for Vim / Neovim

Syntax highlighting and filetype detection for the wisp language. Highlighting
works on its own. The language server (`wisp-lsp`) is optional and adds
diagnostics, formatting, document symbols, hover, completion, go-to-definition,
find-references, rename, signature help, and quick-fix code actions.

## Install the highlighting

Put `syntax/wisp.vim` and `ftdetect/wisp.vim` on your `runtimepath`.

- Manual: copy this directory's contents into `~/.vim/` (Vim) or
  `~/.config/nvim/` (Neovim), so you have `~/.vim/syntax/wisp.vim` and
  `~/.vim/ftdetect/wisp.vim`.
- Plugin manager: point it at this `editors/vim` directory. For example with
  vim-plug:

  ```vim
  Plug '/path/to/wisp/editors/vim'
  ```

Open a `.wisp` file; `ftdetect/wisp.vim` sets the filetype and the syntax loads.

## Build and install the language server (optional)

```sh
# from the repository root
go build -o wisp-lsp ./cmd/wisp-lsp
mv wisp-lsp /usr/local/bin/   # or anywhere on your PATH
# or: go install github.com/mitchellnemitz/wisp/cmd/wisp-lsp@latest
```

## Wire up the language server (optional)

`wisp-lsp` speaks LSP over stdio. Launch it with no arguments.

Neovim built-in LSP:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "wisp",
  callback = function(args)
    vim.lsp.start({
      name = "wisp-lsp",
      cmd = { "wisp-lsp" },
      root_dir = vim.fs.dirname(args.file),
    })
  end,
})
```

With nvim-lspconfig you can register a custom server whose `cmd` is
`{ "wisp-lsp" }` and `filetypes` is `{ "wisp" }`. With coc.nvim, add an entry
under `languageserver` in `coc-settings.json` with `"command": "wisp-lsp"` and
`"filetypes": ["wisp"]`.
