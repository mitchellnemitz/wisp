package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

func TestParseEnumDeclImplicit(t *testing.T) {
	prog := parseOK(t, "enum Color { Red, Green, Blue }\nfn main() -> int { return 0 }")
	if len(prog.Enums) != 1 {
		t.Fatalf("expected 1 enum, got %d", len(prog.Enums))
	}
	ed := prog.Enums[0]
	if ed.Name != "Color" {
		t.Fatalf("enum name = %q, want Color", ed.Name)
	}
	if len(ed.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(ed.Variants))
	}
	wantNames := []string{"Red", "Green", "Blue"}
	for i, v := range ed.Variants {
		if v.Name != wantNames[i] {
			t.Errorf("variant %d name = %q, want %q", i, v.Name, wantNames[i])
		}
		if v.Value != nil {
			t.Errorf("variant %d value = %T, want nil (implicit)", i, v.Value)
		}
		if v.NamePos.Line == 0 {
			t.Errorf("variant %d NamePos unset", i)
		}
	}
	if ed.KwPos.Line != 1 {
		t.Errorf("KwPos line = %d, want 1", ed.KwPos.Line)
	}
	if ed.NamePos.Line != 1 {
		t.Errorf("NamePos line = %d, want 1", ed.NamePos.Line)
	}
	if ed.Multiline {
		t.Errorf("Multiline = true, want false for single-line enum")
	}
}

func TestParseEnumDeclExplicitValue(t *testing.T) {
	prog := parseOK(t, "enum E { A = 5, B }\nfn main() -> int { return 0 }")
	ed := prog.Enums[0]
	if len(ed.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(ed.Variants))
	}
	if ed.Variants[0].Name != "A" {
		t.Fatalf("variant 0 name = %q, want A", ed.Variants[0].Name)
	}
	lit, ok := ed.Variants[0].Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("variant 0 value = %T, want *ast.IntLit", ed.Variants[0].Value)
	}
	if lit.Raw != "5" {
		t.Errorf("variant 0 value raw = %q, want 5", lit.Raw)
	}
	if ed.Variants[1].Value != nil {
		t.Errorf("variant 1 value = %T, want nil", ed.Variants[1].Value)
	}
}

func TestParseEnumDeclNegativeValue(t *testing.T) {
	prog := parseOK(t, "enum E2 { X = -1, Y }\nfn main() -> int { return 0 }")
	ed := prog.Enums[0]
	un, ok := ed.Variants[0].Value.(*ast.UnaryExpr)
	if !ok {
		t.Fatalf("variant 0 value = %T, want *ast.UnaryExpr", ed.Variants[0].Value)
	}
	if _, ok := un.X.(*ast.IntLit); !ok {
		t.Fatalf("unary operand = %T, want *ast.IntLit", un.X)
	}
	if ed.Variants[1].Value != nil {
		t.Errorf("variant 1 value = %T, want nil", ed.Variants[1].Value)
	}
}

func TestParseEnumMultilineTrailingComma(t *testing.T) {
	prog := parseOK(t, "enum State {\n    Idle,\n    Running,\n    Done,\n}\nfn main() -> int { return 0 }")
	ed := prog.Enums[0]
	if !ed.Multiline {
		t.Errorf("Multiline = false, want true for newline-separated enum")
	}
	if len(ed.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(ed.Variants))
	}
}

func TestParseEnumSingleLineNotMultiline(t *testing.T) {
	prog := parseOK(t, "enum Color { Red, Green, Blue }\nfn main() -> int { return 0 }")
	if prog.Enums[0].Multiline {
		t.Errorf("Multiline = true, want false")
	}
}

func TestParseEnumEmptyError(t *testing.T) {
	err := parseErr(t, "enum E {}\nfn main() -> int { return 0 }")
	if !strings.Contains(err.Error(), "enum") {
		t.Errorf("error = %q, want it to mention enum", err.Error())
	}
}

func TestParseExportEnumRejected(t *testing.T) {
	err := parseErr(t, "export enum S { Idle }\nfn main() -> int { return 0 }")
	if !strings.Contains(err.Error(), "not yet supported") {
		t.Errorf("error = %q, want 'not yet supported'", err.Error())
	}
}

func TestParseEnumDoesNotBreakOtherDecls(t *testing.T) {
	prog := parseOK(t, "struct Point { x: int, y: int }\nconst N: int = 3\nfn main() -> int { return 0 }")
	if len(prog.Structs) != 1 {
		t.Errorf("expected 1 struct, got %d", len(prog.Structs))
	}
	if len(prog.Consts) != 1 {
		t.Errorf("expected 1 const, got %d", len(prog.Consts))
	}
	if len(prog.Funcs) != 1 {
		t.Errorf("expected 1 func, got %d", len(prog.Funcs))
	}
	if len(prog.Enums) != 0 {
		t.Errorf("expected 0 enums, got %d", len(prog.Enums))
	}
}
