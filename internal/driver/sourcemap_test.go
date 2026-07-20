package driver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/codegen"
)

// decodeMap parses serialized source-map bytes into a typed value plus the raw
// `lines` array (so a null entry is distinguishable from a zero {l,c}).
func decodeMap(t *testing.T, b []byte) (int, string, string, []*lineEntry) {
	t.Helper()
	var raw struct {
		Version   int          `json:"version"`
		Generated string       `json:"generated"`
		Source    string       `json:"source"`
		Lines     []*lineEntry `json:"lines"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("source map is not valid JSON: %v\n%s", err, b)
	}
	return raw.Version, raw.Generated, raw.Source, raw.Lines
}

// TestSourceMapValidJSONVersion1 asserts the map is valid JSON with version 1
// and a dense lines array (one entry per generated line).
func TestSourceMapValidJSONVersion1(t *testing.T) {
	script, lm, diags := Compile("in.wisp", helloSrc)
	if hasError(diags) {
		t.Fatalf("compile: %v", diags)
	}
	b, err := buildSourceMap("out.sh", "in.wisp", script, lm)
	if err != nil {
		t.Fatal(err)
	}
	version, gen, source, lines := decodeMap(t, b)
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
	if gen != "out.sh" {
		t.Errorf("generated = %q, want out.sh", gen)
	}
	if source != "in.wisp" {
		t.Errorf("source = %q, want in.wisp", source)
	}
	wantLines := countLines(script)
	if len(lines) != wantLines {
		t.Errorf("lines length = %d, want %d (one per generated line)", len(lines), wantLines)
	}
}

func countLines(script []byte) int {
	s := string(script)
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// TestSourceMapRoundTrip asserts re-encoding the decoded map reproduces it.
func TestSourceMapRoundTrip(t *testing.T) {
	script, lm, _ := Compile("in.wisp", helloSrc)
	b, err := buildSourceMap("out.sh", "in.wisp", script, lm)
	if err != nil {
		t.Fatal(err)
	}
	var sm sourceMap
	if err := json.Unmarshal(b, &sm); err != nil {
		t.Fatal(err)
	}
	b2, err := buildSourceMap(sm.Generated, sm.Source, script, lm)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(b2) {
		t.Fatalf("round-trip differs:\n--1--\n%s\n--2--\n%s", b, b2)
	}
}

// TestSourceMapExhaustive asserts every lines entry matches the expected wisp
// l:c / null for one program. The map mirrors codegen's per-line table, so this
// re-pins the §3.2 projection of the table verified exhaustively in codegen.
func TestSourceMapExhaustive(t *testing.T) {
	src := "fn main() -> int {\n" + // 1
		"  let a: int = 7\n" + // 2
		"  print(\"${a}\")\n" + // 3
		"  return 0\n" + // 4
		"}\n" // 5
	script, lm, diags := Compile("p.wisp", src)
	if hasError(diags) {
		t.Fatalf("compile: %v", diags)
	}
	b, err := buildSourceMap("p.sh", "p.wisp", script, lm)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, lines := decodeMap(t, b)

	// Same expected map as the codegen exhaustive test, projected to {l,c}/null.
	// 0 means null; >0 is the expected wisp line. Columns are checked against the
	// codegen table directly below.
	wantLine := []int{
		0, 0, 0, 0, 0, // banner: shebang, comment, shellcheck, zsh shim, blank
		0, 0, 0, 0, 0, 0, 0,
		0,
		1, 1, 1,
		2,
		3, 3,
		4, 4,
		1,
		0,
		0,
	}
	if len(lines) != len(wantLine) {
		t.Fatalf("lines=%d want=%d", len(lines), len(wantLine))
	}
	for i, w := range wantLine {
		switch {
		case w == 0:
			if lines[i] != nil {
				t.Errorf("line %d: got %v, want null", i+1, lines[i])
			}
		case lines[i] == nil:
			t.Errorf("line %d: got null, want l=%d", i+1, w)
		case lines[i].L != w:
			t.Errorf("line %d: got l=%d, want l=%d", i+1, lines[i].L, w)
		}
	}

	// The serialized {l,c} must match the codegen table exactly (column too).
	for i, p := range lm {
		if p == nil {
			if lines[i] != nil {
				t.Errorf("line %d: map null but serialized %v", i+1, lines[i])
			}
			continue
		}
		if lines[i] == nil {
			t.Errorf("line %d: serialized null but map %v", i+1, *p)
			continue
		}
		if lines[i].L != p.Line || lines[i].C != p.Col {
			t.Errorf("line %d: serialized {l:%d,c:%d}, map {l:%d,c:%d}", i+1, lines[i].L, lines[i].C, p.Line, p.Col)
		}
	}
}

// TestSourceMapNullDistinctFromZero ensures a null entry serializes as JSON null
// (not {"l":0,"c":0}) so consumers can tell "no origin" from a real position.
func TestSourceMapNullDistinctFromZero(t *testing.T) {
	lm := []*codegen.SourcePos{nil, {File: "f", Line: 12, Col: 3}}
	b, err := buildSourceMap("g.sh", "f.wisp", []byte("a\nb\n"), lm)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "null") {
		t.Errorf("expected a JSON null for the no-origin line:\n%s", b)
	}
	_, _, _, lines := decodeMap(t, b)
	if lines[0] != nil {
		t.Errorf("entry 0 = %v, want null", lines[0])
	}
	if lines[1] == nil || lines[1].L != 12 || lines[1].C != 3 {
		t.Errorf("entry 1 = %v, want {l:12,c:3}", lines[1])
	}
}
