// Command wisp-lsp is the wisp Language Server. It speaks LSP (JSON-RPC 2.0)
// over stdio and reuses the wisp compiler's lexer, parser, checker, and
// formatter -- it never reimplements language logic. Editors wire it up as the
// language server for `.wisp` files; see editors/ for VSCode and Vim configs.
//
// It builds as a static binary (CGO_ENABLED=0), so it runs in the same minimal
// environments as the compiler.
package main

import (
	"os"

	"github.com/mitchellnemitz/wisp/internal/lsp"
)

func main() {
	os.Exit(lsp.Serve(os.Stdin, os.Stdout, os.Stderr))
}
