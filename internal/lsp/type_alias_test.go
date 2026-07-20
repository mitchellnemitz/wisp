package lsp

import (
	"strings"
	"testing"
)

func TestDocumentSymbolTypeAlias(t *testing.T) {
	src := "type Miles = int\nfn main() -> int {\n  return 0\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/documentSymbol", uri))
	resp := respForID(msgs, 2)
	syms, _ := resp["result"].([]any)
	if len(syms) != 2 {
		t.Fatalf("want 2 symbols (alias + main), got %v", resp["result"])
	}
	alias := syms[0].(map[string]any)
	if alias["name"] != "Miles" || alias["kind"] != float64(symbolKindTypeParameter) {
		t.Errorf("symbol 0 = %v, want Miles/TypeParameter(%d)", alias, symbolKindTypeParameter)
	}
	// selectionRange covers the name "Miles": "type " is 5 chars, so chars 5..10.
	sel := alias["selectionRange"].(map[string]any)
	start := sel["start"].(map[string]any)
	end := sel["end"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(5) || end["character"] != float64(10) {
		t.Errorf("Miles selectionRange = %v..%v, want char 5..10 on line 0", start, end)
	}
}

func TestHoverAndCompletionTypeAlias(t *testing.T) {
	src := "type Miles = int\n" +
		"fn main() -> int {\n" +
		"  let m: Miles = 5\n" +
		"  return m\n" +
		"}\n"
	// Hover on the `Miles` annotation (line 2, "  let m: " is 9 chars -> char 9).
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/hover", uri, 2, 9))
	resp := respForID(msgs, 2)
	hv, _ := resp["result"].(map[string]any)
	if hv == nil {
		t.Fatal("hover result is null, want alias hover text")
	}
	val, _ := hv["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(val, "type Miles = int") {
		t.Errorf("hover value = %q, want `type Miles = int`", val)
	}

	// Completion includes the alias name.
	_, msgs = runServer(didOpenFrame(uri, src) + posReqFrame(3, "textDocument/completion", uri, 3, 9))
	resp = respForID(msgs, 3)
	items, _ := resp["result"].([]any)
	found := false
	for _, it := range items {
		m := it.(map[string]any)
		if m["label"] == "Miles" {
			found = true
			if m["detail"] != "type alias" {
				t.Errorf("Miles completion detail = %v, want \"type alias\"", m["detail"])
			}
		}
	}
	if !found {
		t.Error("completion missing type alias \"Miles\"")
	}
}
