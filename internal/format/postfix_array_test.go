package format

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// typeOfLet parses `fn main() -> int { let x: <ann> = 0 return 0 }` and returns
// the parsed type annotation. Value need not typecheck; Parse is parse-only.
func typeOfLet(t *testing.T, ann string) ast.TypeName {
	t.Helper()
	src := "fn main() -> int {\n  let x: " + ann + " = 0\n  return 0\n}\n"
	prog, err := parser.Parse(src, "t.wisp")
	if err != nil {
		t.Fatalf("Parse(%q): %v", ann, err)
	}
	return prog.Funcs[0].Body[0].(*ast.LetStmt).Type
}

// TestFormatTypePostfix pins that formatType emits postfix `T[]` and
// parenthesizes a funcref element so `[]` cannot rebind.
func TestFormatTypePostfix(t *testing.T) {
	cases := []struct{ ann, want string }{
		{"int[]", "int[]"},
		{"int[][]", "int[][]"},
		{"(int, string)[]", "(int, string)[]"},
		{"{string: int[]}", "{string: int[]}"},
		{"Box[int][]", "Box[int][]"},
		{"Optional[int[]]", "Optional[int[]]"},
		// The critical parenthesization: array-of-funcref must keep its parens.
		{"(fn(int) -> string)[]", "(fn(int) -> string)[]"},
		// ...distinct from a funcref returning an array (no parens).
		{"fn(int) -> string[]", "fn(int) -> string[]"},
	}
	for _, c := range cases {
		src := "fn main() -> int {\n  let x: " + c.ann + " = 0\n  return 0\n}\n"
		out := mustFormat(t, src)
		wantLine := "let x: " + c.want + " = 0"
		if !strings.Contains(out, wantLine) {
			t.Errorf("format(%q): missing %q in:\n%s", c.ann, wantLine, out)
		}
	}
}

// TestFormatTypeRoundTrip: parse -> format -> parse yields the identical
// internal type encoding. This is the guard against a missing funcref paren
// silently turning array-of-funcref into funcref-returning-array.
func TestFormatTypeRoundTrip(t *testing.T) {
	anns := []string{
		"int[]", "int[][]", "int[][][]", "(int, string)[]",
		"{string: int[]}", "Box[int][]", "Optional[int[]]",
		"(fn(int) -> string)[]", "fn(int) -> string[]",
	}
	for _, ann := range anns {
		orig := typeOfLet(t, ann)
		src := "fn main() -> int {\n  let x: " + ann + " = 0\n  return 0\n}\n"
		out := mustFormat(t, src)
		// Re-parse the formatted output and compare the type encoding.
		reprog, err := parser.Parse(out, "t.wisp")
		if err != nil {
			t.Fatalf("re-parse formatted %q: %v\n%s", ann, err, out)
		}
		got := reprog.Funcs[0].Body[0].(*ast.LetStmt).Type
		if got != orig {
			t.Errorf("round-trip %q: parsed %q, reparsed %q", ann, orig, got)
		}
	}
}
