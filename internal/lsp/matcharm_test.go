package lsp

import "testing"

// matchArmProg binds x in a match arm and uses it. Positions (0-indexed):
//
//	line 3: "    case Some(x) {"   binding x at char 14
//	line 4: "      let y: int = x"  use of x at char 19
const matchArmProg = "fn main() -> int {\n" +
	"  let o: Optional[int] = Some(1)\n" +
	"  match (o) {\n" +
	"    case Some(x) {\n" +
	"      let y: int = x\n" +
	"    }\n" +
	"    case None {\n" +
	"    }\n" +
	"  }\n" +
	"  return 0\n" +
	"}\n"

func TestMatchArmVarsDefinition(t *testing.T) {
	// Cursor on the use of x (line 4) jumps to the match arm binding (line 3).
	_, msgs := runServer(didOpenFrame(uri, matchArmProg) + posReqFrame(2, "textDocument/definition", uri, 4, 19))
	loc, _ := respForID(msgs, 2)["result"].(map[string]any)
	if loc == nil {
		t.Fatalf("no definition result for match arm binding")
	}
	start := loc["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(3) || start["character"] != float64(14) {
		t.Errorf("definition pos = line %v char %v, want line 3 char 14 (the match arm binding x)", start["line"], start["character"])
	}
}

func TestMatchArmVarsRename(t *testing.T) {
	// Rename from the use of x (line 4 char 19) -> z: binding + use = 2 edits.
	_, msgs := runServer(didOpenFrame(uri, matchArmProg) + renameFrame(2, uri, 4, 19, "z"))
	we, _ := respForID(msgs, 2)["result"].(map[string]any)
	if we == nil {
		t.Fatalf("no rename result for match arm binding")
	}
	edits, _ := we["changes"].(map[string]any)[uri].([]any)
	if len(edits) != 2 {
		t.Fatalf("rename edits = %d, want 2 (binding + use): %v", len(edits), edits)
	}
	gotLines := map[float64]bool{}
	for _, e := range edits {
		em := e.(map[string]any)
		if em["newText"] != "z" {
			t.Errorf("edit newText = %v, want z", em["newText"])
		}
		gotLines[em["range"].(map[string]any)["start"].(map[string]any)["line"].(float64)] = true
	}
	if !gotLines[3] || !gotLines[4] {
		t.Errorf("rename edits on lines %v, want lines 3 and 4", gotLines)
	}
}
