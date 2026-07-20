package lsp

import "testing"

// blankProg binds `_` at two sites (0-indexed positions):
//
//	line 2: "  let _: int = 5"        the `_` is at char 6
//	line 3: "  for (_ in xs) {"       the `_` is at char 7
const blankProg = "fn main() -> int {\n" +
	"  let xs: int[] = [1]\n" +
	"  let _: int = 5\n" +
	"  for (_ in xs) {\n" +
	"    print(\"x\")\n" +
	"  }\n" +
	"  return 0\n" +
	"}\n"

// hasNavResult reports whether a definition `result` is a non-empty navigable
// result in any LSP success shape (single Location object, or an array of
// Location/LocationLink).
func hasNavResult(result any) bool {
	switch r := result.(type) {
	case nil:
		return false
	case map[string]any:
		return len(r) > 0
	case []any:
		return len(r) > 0
	default:
		return result != nil
	}
}

// hasRenameEdits reports whether a rename `result` carries any edits, via either
// the `changes` map or the `documentChanges` array.
func hasRenameEdits(result any) bool {
	we, ok := result.(map[string]any)
	if !ok || we == nil {
		return false
	}
	if changes, ok := we["changes"].(map[string]any); ok && len(changes) > 0 {
		return true
	}
	if dc, ok := we["documentChanges"].([]any); ok && len(dc) > 0 {
		return true
	}
	return false
}

func TestBlankLetNotNavigable(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, blankProg) + posReqFrame(2, "textDocument/definition", uri, 2, 6))
	if hasNavResult(respForID(msgs, 2)["result"]) {
		t.Errorf("definition on blank `let _` returned a result, want none")
	}
}

func TestBlankForInNotNavigable(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, blankProg) + posReqFrame(2, "textDocument/definition", uri, 3, 7))
	if hasNavResult(respForID(msgs, 2)["result"]) {
		t.Errorf("definition on blank `for (_ in ...)` returned a result, want none")
	}
}

func TestBlankLetNotRenameable(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, blankProg) + renameFrame(2, uri, 2, 6, "z"))
	if hasRenameEdits(respForID(msgs, 2)["result"]) {
		t.Errorf("rename on blank `let _` produced edits, want none")
	}
}

func TestBlankForInNotRenameable(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, blankProg) + renameFrame(2, uri, 3, 7, "z"))
	if hasRenameEdits(respForID(msgs, 2)["result"]) {
		t.Errorf("rename on blank `for (_ in ...)` produced edits, want none")
	}
}
