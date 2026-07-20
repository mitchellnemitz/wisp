package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// errAt asserts err's message contains substr (helper already exists in parser_test.go).
// We reuse the same helper; it is defined in error_test.go as a package-level func.

// --- top-level const ---

func TestTopLevelConstLandsInProgramConsts(t *testing.T) {
	prog := parseOK(t, "const MAX: int = 5")
	if len(prog.Consts) != 1 {
		t.Fatalf("Program.Consts = %d, want 1", len(prog.Consts))
	}
	cd := prog.Consts[0]
	if cd.Name != "MAX" {
		t.Errorf("Name = %q, want MAX", cd.Name)
	}
	if cd.Type != ast.TypeInt {
		t.Errorf("Type = %q, want int", cd.Type)
	}
	lit, ok := cd.Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("Value = %T, want *ast.IntLit", cd.Value)
	}
	if lit.Raw != "5" {
		t.Errorf("Value.Raw = %q, want 5", lit.Raw)
	}
}

func TestTopLevelConstDoesNotLandInFuncsOrStructs(t *testing.T) {
	prog := parseOK(t, "const X: bool = true")
	if len(prog.Funcs) != 0 || len(prog.Structs) != 0 {
		t.Errorf("top-level const leaked into Funcs or Structs: funcs=%d structs=%d",
			len(prog.Funcs), len(prog.Structs))
	}
}

func TestMultipleTopLevelConsts(t *testing.T) {
	prog := parseOK(t, "const A: int = 1\nconst B: string = 'hi'")
	if len(prog.Consts) != 2 {
		t.Fatalf("Program.Consts = %d, want 2", len(prog.Consts))
	}
	if prog.Consts[0].Name != "A" || prog.Consts[1].Name != "B" {
		t.Errorf("names = %q, %q; want A, B", prog.Consts[0].Name, prog.Consts[1].Name)
	}
}

func TestTopLevelConstPosition(t *testing.T) {
	prog := parseOK(t, "const MAX: int = 42")
	cd := prog.Consts[0]
	if cd.KwPos.Col == 0 {
		t.Errorf("KwPos.Col == 0; expected a real position")
	}
	if cd.NamePos.Col == 0 {
		t.Errorf("NamePos.Col == 0; expected a real position")
	}
}

// --- top-level final errors ---

func TestTopLevelFinalIsError(t *testing.T) {
	err := parseErr(t, "final X: int = 5")
	if err == nil {
		t.Fatal("expected error for top-level final, got nil")
	}
	if !strings.Contains(err.Error(), "final") {
		t.Errorf("error %q should mention 'final'", err.Error())
	}
}

// --- body const ---

func TestBodyConstStatement(t *testing.T) {
	prog := parseOK(t, wrap("const x: int = 99"))
	body := mainBody(t, prog)
	if len(body) != 1 {
		t.Fatalf("body len = %d, want 1", len(body))
	}
	cs, ok := body[0].(*ast.ConstStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *ast.ConstStmt", body[0])
	}
	if cs.Name != "x" {
		t.Errorf("Name = %q, want x", cs.Name)
	}
	if cs.Type != ast.TypeInt {
		t.Errorf("Type = %q, want int", cs.Type)
	}
	lit, ok := cs.Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("Value = %T, want *ast.IntLit", cs.Value)
	}
	if lit.Raw != "99" {
		t.Errorf("Value.Raw = %q, want 99", lit.Raw)
	}
}

func TestBodyConstStringType(t *testing.T) {
	prog := parseOK(t, wrap("const S: string = 'hello'"))
	body := mainBody(t, prog)
	cs := body[0].(*ast.ConstStmt)
	if cs.Type != ast.TypeString {
		t.Errorf("Type = %q, want string", cs.Type)
	}
}

// --- body final ---

func TestBodyFinalStatement(t *testing.T) {
	prog := parseOK(t, wrap("final y: int = 7"))
	body := mainBody(t, prog)
	if len(body) != 1 {
		t.Fatalf("body len = %d, want 1", len(body))
	}
	fs, ok := body[0].(*ast.FinalStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *ast.FinalStmt", body[0])
	}
	if fs.Name != "y" {
		t.Errorf("Name = %q, want y", fs.Name)
	}
	if fs.Type != ast.TypeInt {
		t.Errorf("Type = %q, want int", fs.Type)
	}
	lit, ok := fs.Value.(*ast.IntLit)
	if !ok {
		t.Fatalf("Value = %T, want *ast.IntLit", fs.Value)
	}
	if lit.Raw != "7" {
		t.Errorf("Value.Raw = %q, want 7", lit.Raw)
	}
}

func TestBodyFinalBoolType(t *testing.T) {
	prog := parseOK(t, wrap("final flag: bool = true"))
	body := mainBody(t, prog)
	fs := body[0].(*ast.FinalStmt)
	if fs.Type != ast.TypeBool {
		t.Errorf("Type = %q, want bool", fs.Type)
	}
}

func TestBodyFinalPosition(t *testing.T) {
	prog := parseOK(t, wrap("final z: int = 0"))
	body := mainBody(t, prog)
	fs := body[0].(*ast.FinalStmt)
	if fs.KwPos.Col == 0 {
		t.Errorf("KwPos.Col == 0; expected a real position")
	}
	if fs.NamePos.Col == 0 {
		t.Errorf("NamePos.Col == 0; expected a real position")
	}
}

// --- missing annotation errors ---

func TestConstMissingAnnotationIsError(t *testing.T) {
	err := parseErr(t, wrap("const x = 5"))
	if err == nil {
		t.Fatal("expected error for const without type annotation, got nil")
	}
	// Should complain about missing ':' (the annotation separator)
	if !strings.Contains(err.Error(), ":") && !strings.Contains(strings.ToLower(err.Error()), "expected") {
		t.Errorf("error %q should indicate missing annotation", err.Error())
	}
}

func TestFinalMissingAnnotationIsError(t *testing.T) {
	err := parseErr(t, wrap("final x = 5"))
	if err == nil {
		t.Fatal("expected error for final without type annotation, got nil")
	}
}

func TestTopLevelConstMissingAnnotationIsError(t *testing.T) {
	err := parseErr(t, "const MAX = 5")
	if err == nil {
		t.Fatal("expected error for top-level const without type annotation, got nil")
	}
}

// --- final rejected as identifier ---

func TestFinalRejectedAsIdentifier(t *testing.T) {
	// `final` must be a keyword, not usable as a variable name
	err := parseErr(t, wrap("let final: int = 1"))
	if err == nil {
		t.Fatal("expected error when 'final' used as identifier, got nil")
	}
}

// --- export final not parsed ---

func TestExportFinalIsError(t *testing.T) {
	err := parseErr(t, "export final X: int = 5")
	if err == nil {
		t.Fatal("expected error for 'export final' (not in this PR), got nil")
	}
}

// --- const and fn together ---

func TestTopLevelConstWithFunc(t *testing.T) {
	src := "const MAX: int = 100\n\nfn main() -> void {\n  return\n}"
	prog := parseOK(t, src)
	if len(prog.Consts) != 1 {
		t.Fatalf("Consts = %d, want 1", len(prog.Consts))
	}
	if len(prog.Funcs) != 1 {
		t.Fatalf("Funcs = %d, want 1", len(prog.Funcs))
	}
}

// --- ConstDecl position fields ---

func TestConstDeclKwPos(t *testing.T) {
	prog := parseOK(t, "const FOO: int = 0")
	cd := prog.Consts[0]
	// KwPos.Col should be 1 (first column)
	if cd.KwPos.Col != 1 {
		t.Errorf("KwPos.Col = %d, want 1", cd.KwPos.Col)
	}
}

// TestTopLevelParseErrorMentionsConst verifies the top-level parse-error message
// lists const among the valid declarations, since const is now accepted at
// module scope.
func TestTopLevelParseErrorMentionsConst(t *testing.T) {
	err := parseErr(t, "let X: int = 5")
	if err == nil {
		t.Fatal("expected a parse error for `let` at module scope")
	}
	if !strings.Contains(err.Error(), "const") {
		t.Errorf("top-level parse error %q should mention const", err.Error())
	}
}
