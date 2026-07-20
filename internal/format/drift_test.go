package format

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

func TestSimpleStmtText_UnhandledStmtPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "simpleStmtText") || !strings.Contains(msg, "parser/format drift") {
			t.Fatalf("panic message %q missing expected substrings", msg)
		}
	}()
	p := &printer{}
	p.simpleStmtText(&ast.IfStmt{}, 0)
}

func TestFormatProgram_RecoversPanic(t *testing.T) {
	prog := &ast.Program{
		Funcs: []*ast.FuncDecl{
			{
				Body: []ast.Stmt{
					&ast.ForStmt{Init: &ast.IfStmt{}},
				},
			},
		},
	}
	result, err := formatProgram(prog, nil)
	if err == nil {
		t.Fatalf("expected error, got result %q", result)
	}
	if result != "" {
		t.Fatalf("expected empty result on error, got %q", result)
	}
	if !strings.Contains(err.Error(), "simpleStmtText") {
		t.Fatalf("error message %q missing %q", err.Error(), "simpleStmtText")
	}
}

func TestMatchPattern_UnhandledPatternPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "matchPattern") || !strings.Contains(msg, "parser/format drift") {
			t.Fatalf("panic message %q missing expected substrings", msg)
		}
	}()
	p := &printer{}
	p.matchPattern(nil)
}

func TestExpr_UnhandledNodePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "expr") || !strings.Contains(msg, "parser/format drift") {
			t.Fatalf("panic message %q missing expected substrings", msg)
		}
	}()
	p := &printer{}
	p.expr(nil, 0)
}
