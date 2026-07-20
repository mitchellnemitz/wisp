package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// parseFileOK parses src under the given filename, expecting success.
func parseFileOK(t *testing.T, src, filename string) *ast.Program {
	t.Helper()
	prog, err := Parse(src, filename)
	if err != nil {
		t.Fatalf("Parse(%q, %q) unexpected error: %v", src, filename, err)
	}
	return prog
}

// parseFileErr parses src under the given filename, expecting an error.
func parseFileErr(t *testing.T, src, filename string) error {
	t.Helper()
	_, err := Parse(src, filename)
	if err == nil {
		t.Fatalf("Parse(%q, %q): expected error, got none", src, filename)
	}
	return err
}

func TestTestDeclParses(t *testing.T) {
	prog := parseFileOK(t, `test ("x") {
}`, "foo_test.wisp")
	if len(prog.Tests) != 1 {
		t.Fatalf("tests = %d, want 1", len(prog.Tests))
	}
	td := prog.Tests[0]
	if td.Name != "x" {
		t.Errorf("name = %q, want x", td.Name)
	}
	if td.Pos().Line != 1 || td.Pos().Col != 1 {
		t.Errorf("test pos = %v, want 1:1", td.Pos())
	}
	if len(td.Body) != 0 {
		t.Errorf("body stmts = %d, want 0", len(td.Body))
	}
}

func TestTestDeclWithBody(t *testing.T) {
	prog := parseFileOK(t, `test ("does a thing") {
  let n: int = 1
}`, "foo_test.wisp")
	if len(prog.Tests) != 1 {
		t.Fatalf("tests = %d, want 1", len(prog.Tests))
	}
	td := prog.Tests[0]
	if td.Name != "does a thing" {
		t.Errorf("name = %q, want %q", td.Name, "does a thing")
	}
	if len(td.Body) != 1 {
		t.Fatalf("body stmts = %d, want 1", len(td.Body))
	}
	if _, ok := td.Body[0].(*ast.LetStmt); !ok {
		t.Errorf("body[0] = %T, want *ast.LetStmt", td.Body[0])
	}
}

func TestTestDeclSourceOrder(t *testing.T) {
	prog := parseFileOK(t, `test ("a") {
}
test ("b") {
}
test ("c") {
}`, "foo_test.wisp")
	if len(prog.Tests) != 3 {
		t.Fatalf("tests = %d, want 3", len(prog.Tests))
	}
	for i, want := range []string{"a", "b", "c"} {
		if prog.Tests[i].Name != want {
			t.Errorf("tests[%d].Name = %q, want %q", i, prog.Tests[i].Name, want)
		}
	}
}

func TestTestDeclOutsideTestFileIsError(t *testing.T) {
	err := parseFileErr(t, `test ("x") {
}`, "foo.wisp")
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("error = %q, want it to mention `test`", err.Error())
	}
}

func TestTestDeclNoParensIsError(t *testing.T) {
	parseFileErr(t, `test x {
}`, "foo_test.wisp")
}

func TestTestDeclNoNameIsError(t *testing.T) {
	parseFileErr(t, `test () {
}`, "foo_test.wisp")
}

func TestTestDeclNonStringHeadIsError(t *testing.T) {
	parseFileErr(t, `test (42) {
}`, "foo_test.wisp")
}
