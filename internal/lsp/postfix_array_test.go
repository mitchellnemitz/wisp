package lsp

import (
	"strings"
	"testing"
)

// hoverVal opens src and hovers at (line, char), returning the hover string.
func hoverVal(t *testing.T, src string, line, char int) string {
	t.Helper()
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/hover", uri, line, char))
	resp := respForID(msgs, 2)
	if resp == nil {
		t.Fatal("no hover response")
	}
	hv, _ := resp["result"].(map[string]any)
	if hv == nil {
		t.Fatalf("hover result is null at (%d,%d)", line, char)
	}
	contents, _ := hv["contents"].(map[string]any)
	val, _ := contents["value"].(string)
	return val
}

// TestHoverArrayTypesPostfix: LSP hover renders array types in postfix `T[]`
// form (regression guard for server.go:515/520/582/588 raw string(...) sites).
func TestHoverArrayTypesPostfix(t *testing.T) {
	// Line 0: `fn f(xs: int[]) -> int[][] {`  -- funcSignature params + return.
	src := "fn f(xs: int[]) -> int[][] {\n  return [xs]\n}\n" +
		"fn main() -> int {\n  let ys: int[] = [1]\n  print(\"${length(ys)}\")\n  return 0\n}\n"

	// Hover on the definition name `f` at line 0, char 3.
	if v := hoverVal(t, src, 0, 3); !strings.Contains(v, "xs: int[]") || !strings.Contains(v, "-> int[][]") {
		t.Errorf("funcSignature hover = %q, want postfix params/return", v)
	}

	// Hover on the variable use `ys` in `print("${length(ys)}")` (line 5).
	// "  print(\"${length(" = 2 + 6 + 3 + 7 = char 18.
	line5 := "  print(\"${length(ys)}\")"
	col := strings.Index(line5, "ys")
	if v := hoverVal(t, src, 5, col); !strings.Contains(v, "ys: int[]") {
		t.Errorf("variable hover = %q, want %q", v, "ys: int[]")
	}
}
