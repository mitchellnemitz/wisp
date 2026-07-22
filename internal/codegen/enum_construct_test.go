package codegen

import (
	"strings"
	"testing"
)

func TestEnumConstructEmitsVariantTag(t *testing.T) {
	src := `enum Expr { IntLit(int), Unit }
fn main() -> int {
  let a: Expr = Expr.IntLit(3)
  let b: Expr = Expr.Unit
  return 0
}`
	sh := string(compile(t, src)) // compile(t, src) returns []byte (internal/codegen/codegen_test.go:16)
	if !strings.Contains(sh, "=IntLit") {
		t.Errorf("expected the IntLit variant tag in emitted shell:\n%s", sh)
	}
	if !strings.Contains(sh, "=Unit") {
		t.Errorf("expected the Unit variant tag in emitted shell:\n%s", sh)
	}
}
