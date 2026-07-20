package lsp

import (
	"strings"
	"testing"
)

// navProg has a function add called twice and a local x used once, with known
// positions:
//
//	line 0: fn add(a: int, b: int) -> int {   add name at chars 3..6
//	line 4:   let x: int = add(1, 2)          x at char 6, add call at 15..18
//	line 5:   let y: int = add(x, 3)          add call at 15..18, x use at char 19
const navProg = "fn add(a: int, b: int) -> int {\n" +
	"  return a + b\n" +
	"}\n" +
	"fn main() -> int {\n" +
	"  let x: int = add(1, 2)\n" +
	"  let y: int = add(x, 3)\n" +
	"  return y\n" +
	"}\n"

func refFrame(id int, uri string, line, char int, includeDecl bool) string {
	return reqFrame(id, "textDocument/references", mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": char},
		"context":      map[string]any{"includeDeclaration": includeDecl},
	}))
}

func renameFrame(id int, uri string, line, char int, newName string) string {
	return reqFrame(id, "textDocument/rename", mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": char},
		"newName":      newName,
	}))
}

func TestDefinition(t *testing.T) {
	// Cursor inside the add call on line 4 jumps to the add declaration name.
	_, msgs := runServer(didOpenFrame(uri, navProg) + posReqFrame(2, "textDocument/definition", uri, 4, 16))
	resp := respForID(msgs, 2)
	loc, _ := resp["result"].(map[string]any)
	if loc == nil {
		t.Fatalf("no definition result: %v", resp)
	}
	if loc["uri"] != uri {
		t.Errorf("uri = %v, want %s", loc["uri"], uri)
	}
	start := loc["range"].(map[string]any)["start"].(map[string]any)
	end := loc["range"].(map[string]any)["end"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(3) || end["character"] != float64(6) {
		t.Errorf("definition range = %v..%v, want line 0 chars 3..6", start, end)
	}
}

func TestReferences(t *testing.T) {
	// add: declaration + two calls = 3 with the declaration included.
	_, msgs := runServer(didOpenFrame(uri, navProg) + refFrame(2, uri, 0, 4, true))
	locs, _ := respForID(msgs, 2)["result"].([]any)
	if len(locs) != 3 {
		t.Fatalf("references(includeDeclaration) = %d, want 3: %v", len(locs), locs)
	}

	// Without the declaration: just the two call sites.
	_, msgs = runServer(didOpenFrame(uri, navProg) + refFrame(3, uri, 0, 4, false))
	locs, _ = respForID(msgs, 3)["result"].([]any)
	if len(locs) != 2 {
		t.Fatalf("references(excludeDeclaration) = %d, want 2: %v", len(locs), locs)
	}
}

func TestRename(t *testing.T) {
	// Rename the local x (declared line 4 char 6, used line 5 char 19) to z.
	_, msgs := runServer(didOpenFrame(uri, navProg) + renameFrame(2, uri, 4, 6, "z"))
	we, _ := respForID(msgs, 2)["result"].(map[string]any)
	if we == nil {
		t.Fatalf("no rename result")
	}
	edits, _ := we["changes"].(map[string]any)[uri].([]any)
	if len(edits) != 2 {
		t.Fatalf("rename edits = %d, want 2: %v", len(edits), edits)
	}
	gotLines := map[float64]bool{}
	for _, e := range edits {
		em := e.(map[string]any)
		if em["newText"] != "z" {
			t.Errorf("edit newText = %v, want z", em["newText"])
		}
		gotLines[em["range"].(map[string]any)["start"].(map[string]any)["line"].(float64)] = true
	}
	if !gotLines[4] || !gotLines[5] {
		t.Errorf("rename edits on lines %v, want lines 4 and 5", gotLines)
	}
}

func TestSignatureHelp(t *testing.T) {
	// Just after add( on line 4: active parameter 0.
	_, msgs := runServer(didOpenFrame(uri, navProg) + posReqFrame(2, "textDocument/signatureHelp", uri, 4, 19))
	sh, _ := respForID(msgs, 2)["result"].(map[string]any)
	if sh == nil {
		t.Fatalf("no signatureHelp result")
	}
	sigs := sh["signatures"].([]any)
	if len(sigs) != 1 {
		t.Fatalf("signatures = %d, want 1", len(sigs))
	}
	label := sigs[0].(map[string]any)["label"].(string)
	if !strings.Contains(label, "fn add(a: int, b: int) -> int") {
		t.Errorf("signature label = %q", label)
	}
	if sh["activeParameter"] != float64(0) {
		t.Errorf("activeParameter = %v, want 0", sh["activeParameter"])
	}

	// After the first comma: active parameter 1.
	_, msgs = runServer(didOpenFrame(uri, navProg) + posReqFrame(3, "textDocument/signatureHelp", uri, 4, 22))
	sh, _ = respForID(msgs, 3)["result"].(map[string]any)
	if sh == nil || sh["activeParameter"] != float64(1) {
		t.Errorf("activeParameter after comma = %v, want 1", sh["activeParameter"])
	}
}

func TestCodeActionDidYouMean(t *testing.T) {
	// The client sends a did-you-mean diagnostic; the server offers a quick fix
	// that replaces the flagged range with the suggestion.
	params := mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"range": map[string]any{
			"start": map[string]any{"line": 5, "character": 15},
			"end":   map[string]any{"line": 5, "character": 17},
		},
		"context": map[string]any{
			"diagnostics": []map[string]any{{
				"range": map[string]any{
					"start": map[string]any{"line": 5, "character": 15},
					"end":   map[string]any{"line": 5, "character": 17},
				},
				"severity": 1,
				"message":  `unknown function "ad"; did you mean "add"?`,
			}},
		},
	})
	_, msgs := runServer(didOpenFrame(uri, navProg) + reqFrame(2, "textDocument/codeAction", params))
	actions, _ := respForID(msgs, 2)["result"].([]any)
	if len(actions) != 1 {
		t.Fatalf("code actions = %d, want 1: %v", len(actions), actions)
	}
	a := actions[0].(map[string]any)
	if a["title"] != `Change to "add"` {
		t.Errorf("title = %v", a["title"])
	}
	if a["kind"] != "quickfix" {
		t.Errorf("kind = %v, want quickfix", a["kind"])
	}
	edits := a["edit"].(map[string]any)["changes"].(map[string]any)[uri].([]any)
	if len(edits) != 1 || edits[0].(map[string]any)["newText"] != "add" {
		t.Errorf("edit = %v, want one edit with newText add", edits)
	}
}

func TestNavigationCapabilities(t *testing.T) {
	_, msgs := runServer(reqFrame(1, "initialize", "{}"))
	caps := respForID(msgs, 1)["result"].(map[string]any)["capabilities"].(map[string]any)
	for _, c := range []string{"definitionProvider", "referencesProvider", "renameProvider"} {
		if caps[c] != true {
			t.Errorf("%s not advertised", c)
		}
	}
	if _, ok := caps["codeActionProvider"]; !ok {
		t.Error("codeActionProvider not advertised")
	}
	if _, ok := caps["signatureHelpProvider"]; !ok {
		t.Error("signatureHelpProvider not advertised")
	}
}

// constNavProg: a program with a local const binding.
//
//	line 0: fn main() -> int {
//	line 1:   const x: int = 42       x at char 8 (0-indexed)
//	line 2:   return x                x at char 9
//	line 3: }
const constNavProg = "fn main() -> int {\n" +
	"  const x: int = 42\n" +
	"  return x\n" +
	"}\n"

// finalNavProg: a program with a local final binding.
//
//	line 0: fn main() -> int {
//	line 1:   final y: int = 1        y at char 8 (0-indexed)
//	line 2:   return y                y at char 9
//	line 3: }
const finalNavProg = "fn main() -> int {\n" +
	"  final y: int = 1\n" +
	"  return y\n" +
	"}\n"

// TestDefinitionConst: go-to-definition on a local const name lands on the
// name character (char 8), not the keyword (char 2).
func TestDefinitionConst(t *testing.T) {
	// cursor on x at line 1, char 8 (the declaration)
	_, msgs := runServer(didOpenFrame(uri, constNavProg) + posReqFrame(2, "textDocument/definition", uri, 1, 8))
	resp := respForID(msgs, 2)
	loc, _ := resp["result"].(map[string]any)
	if loc == nil {
		t.Fatalf("no definition result: %v", resp)
	}
	start := loc["range"].(map[string]any)["start"].(map[string]any)
	end := loc["range"].(map[string]any)["end"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(8) || end["character"] != float64(9) {
		t.Errorf("definition range = line %v char %v..%v, want line 1 chars 8..9", start["line"], start["character"], end["character"])
	}
}

// TestDefinitionConstUse: go-to-definition on a use of a local const also
// lands on the name at the declaration, not the keyword.
func TestDefinitionConstUse(t *testing.T) {
	// cursor on x at line 2, char 9 (the use)
	_, msgs := runServer(didOpenFrame(uri, constNavProg) + posReqFrame(2, "textDocument/definition", uri, 2, 9))
	resp := respForID(msgs, 2)
	loc, _ := resp["result"].(map[string]any)
	if loc == nil {
		t.Fatalf("no definition result from use: %v", resp)
	}
	start := loc["range"].(map[string]any)["start"].(map[string]any)
	end := loc["range"].(map[string]any)["end"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(8) || end["character"] != float64(9) {
		t.Errorf("definition-from-use range = line %v char %v..%v, want line 1 chars 8..9", start["line"], start["character"], end["character"])
	}
}

// TestRenameConst: rename on a local const binding updates the declaration and
// its use, and ranges target the name (char 8 on decl line, char 9 on use line).
func TestRenameConst(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, constNavProg) + renameFrame(2, uri, 1, 8, "limit"))
	we, _ := respForID(msgs, 2)["result"].(map[string]any)
	if we == nil {
		t.Fatalf("no rename result")
	}
	edits, _ := we["changes"].(map[string]any)[uri].([]any)
	if len(edits) != 2 {
		t.Fatalf("rename edits = %d, want 2: %v", len(edits), edits)
	}
	for _, e := range edits {
		em := e.(map[string]any)
		if em["newText"] != "limit" {
			t.Errorf("edit newText = %v, want limit", em["newText"])
		}
		// Each edit must start at char 8 (decl) or char 9 (use), never at char 2 (keyword).
		sc := em["range"].(map[string]any)["start"].(map[string]any)["character"].(float64)
		if sc != 8 && sc != 9 {
			t.Errorf("edit start character = %v, want 8 (decl) or 9 (use)", sc)
		}
	}
}

// TestDefinitionFinal: go-to-definition on a local final name lands on the
// name character (char 8), not the keyword (char 2).
func TestDefinitionFinal(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, finalNavProg) + posReqFrame(2, "textDocument/definition", uri, 1, 8))
	resp := respForID(msgs, 2)
	loc, _ := resp["result"].(map[string]any)
	if loc == nil {
		t.Fatalf("no definition result: %v", resp)
	}
	start := loc["range"].(map[string]any)["start"].(map[string]any)
	end := loc["range"].(map[string]any)["end"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(8) || end["character"] != float64(9) {
		t.Errorf("definition range = line %v char %v..%v, want line 1 chars 8..9", start["line"], start["character"], end["character"])
	}
}

// TestRenameFinal: rename on a local final binding updates the declaration and
// its use, with ranges targeting the name (not the keyword).
func TestRenameFinal(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, finalNavProg) + renameFrame(2, uri, 1, 8, "z"))
	we, _ := respForID(msgs, 2)["result"].(map[string]any)
	if we == nil {
		t.Fatalf("no rename result")
	}
	edits, _ := we["changes"].(map[string]any)[uri].([]any)
	if len(edits) != 2 {
		t.Fatalf("rename edits = %d, want 2: %v", len(edits), edits)
	}
	for _, e := range edits {
		em := e.(map[string]any)
		if em["newText"] != "z" {
			t.Errorf("edit newText = %v, want z", em["newText"])
		}
		sc := em["range"].(map[string]any)["start"].(map[string]any)["character"].(float64)
		if sc != 8 && sc != 9 {
			t.Errorf("edit start character = %v, want 8 (decl) or 9 (use)", sc)
		}
	}
}

// abuttingProg: a function where b is used in a no-space binary expression a+b
// and in a.b field access context via a struct param, to exercise the abutting
// first-char cursor fix (M9).
//
// Layout (0-indexed lines, 0-indexed chars):
//
//	line 0: fn sum(a: int, b: int) -> int {
//	line 1:   return a+b                       a at char 9, + at char 10, b at char 11
//	line 2: }
//	line 3: struct P { x: int }
//	line 4: fn getx(p: P) -> int {
//	line 5:   return p.x                       p at char 9, . at char 10, x at char 11
//	line 6: }
//	line 7: fn main() -> int {
//	line 8:   return sum(1, 2) + getx(P{x: 0})
//	line 9: }
const abuttingProg = "fn sum(a: int, b: int) -> int {\n" +
	"  return a+b\n" +
	"}\n" +
	"struct P { x: int }\n" +
	"fn getx(p: P) -> int {\n" +
	"  return p.x\n" +
	"}\n" +
	"fn main() -> int {\n" +
	"  return sum(1, 2) + getx(P{x: 0})\n" +
	"}\n"

// TestHoverFirstCharAbutting: hover with the cursor on the first character of b
// in the no-space expression a+b (line 1, char 11) must resolve b and return a
// non-null hover result. Before the fix, + abutted and stole the cursor, leaving
// hover null.
func TestHoverFirstCharAbutting(t *testing.T) {
	// Cursor on first char of b (char 11): hover must be non-null.
	_, msgs := runServer(didOpenFrame(uri, abuttingProg) + posReqFrame(2, "textDocument/hover", uri, 1, 11))
	resp := respForID(msgs, 2)
	if resp == nil {
		t.Fatal("no hover response")
	}
	hv, _ := resp["result"].(map[string]any)
	if hv == nil {
		t.Fatal("hover on first char of b in a+b returned null; want the b variable hover")
	}
	contents, _ := hv["contents"].(map[string]any)
	val, _ := contents["value"].(string)
	if !strings.Contains(val, "b") {
		t.Errorf("hover value = %q, want it to contain b", val)
	}

	// Regression: cursor at end of b in a + b (with space, line 1 char 11 in the
	// spaced variant) still resolves b. Use navProg's "return a + b" on line 1,
	// where b starts at char 11 and ends at char 12 (width 1). Cursor at char 12
	// (the exclusive end of b) with no adjacent next token on that line should
	// fall back to b.
	//
	// navProg line 1 is "  return a + b"; b is at char 13 (0-indexed), end char 14.
	// Cursor at char 14 (one past b) with nothing following -> fallback resolves b.
	_, msgs = runServer(didOpenFrame(uri, navProg) + posReqFrame(3, "textDocument/hover", uri, 1, 14))
	resp = respForID(msgs, 3)
	if resp == nil {
		t.Fatal("no hover response for end-of-b regression")
	}
	hv2, _ := resp["result"].(map[string]any)
	if hv2 == nil {
		t.Fatal("hover at end of b (fallback path) returned null; end-of-identifier regression")
	}
	val2, _ := hv2["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(val2, "b") {
		t.Errorf("end-of-b hover value = %q, want it to contain b", val2)
	}
}

// TestDefinitionFirstCharAbutting: go-to-definition with cursor on the first
// char of b in a+b (line 1, char 11) must resolve b to its declaration.
func TestDefinitionFirstCharAbutting(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, abuttingProg) + posReqFrame(2, "textDocument/definition", uri, 1, 11))
	resp := respForID(msgs, 2)
	loc, _ := resp["result"].(map[string]any)
	if loc == nil {
		t.Fatal("definition on first char of b in a+b returned null; want b's declaration")
	}
	// b is declared as a parameter on line 0, "fn sum(a: int, b: int)":
	// f=0 n=1 ' '=2 s=3 u=4 m=5 (=6 a=7 :=8 ' '=9 i=10 n=11 t=12 ,=13 ' '=14 b=15
	start := loc["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(15) {
		t.Errorf("definition of b: range start = %v, want line 0 char 15", start)
	}
}

// TestReferencesFirstCharAbutting: find-references with cursor on the first
// char of b in a+b resolves b and returns its occurrences.
func TestReferencesFirstCharAbutting(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, abuttingProg) + refFrame(2, uri, 1, 11, true))
	locs, _ := respForID(msgs, 2)["result"].([]any)
	// b appears as a parameter declaration (line 0) and one use (line 1 a+b).
	if len(locs) < 1 {
		t.Fatalf("references on first char of b in a+b = %d locs, want >= 1: %v", len(locs), locs)
	}
}

// TestRenameFirstCharAbutting: rename with cursor on the first char of b in
// a+b must rename b, not the preceding + operator.
func TestRenameFirstCharAbutting(t *testing.T) {
	_, msgs := runServer(didOpenFrame(uri, abuttingProg) + renameFrame(2, uri, 1, 11, "c"))
	we, _ := respForID(msgs, 2)["result"].(map[string]any)
	if we == nil {
		t.Fatal("rename on first char of b in a+b returned null; want b renamed")
	}
	edits, _ := we["changes"].(map[string]any)[uri].([]any)
	if len(edits) == 0 {
		t.Fatal("rename produced no edits")
	}
	for _, e := range edits {
		em := e.(map[string]any)
		if em["newText"] != "c" {
			t.Errorf("edit newText = %v, want c", em["newText"])
		}
	}
}
