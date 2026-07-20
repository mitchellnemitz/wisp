package parser

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// --- M4: function-reference type parsing + callee-as-expression ---

func TestParseFuncType_Simple(t *testing.T) {
	prog := parseOK(t, "fn main() -> int {\n  let f: fn(int, int) -> int = main\n  return 0\n}")
	ls := mainBody(t, prog)[0].(*ast.LetStmt)
	if ls.Type != ast.FuncType([]ast.TypeName{ast.TypeInt, ast.TypeInt}, ast.TypeInt) {
		t.Errorf("type = %q, want fn(int,int)->int", ls.Type)
	}
}

func TestParseFuncType_VoidReturnOK(t *testing.T) {
	prog := parseOK(t, "fn main() -> int {\n  let f: fn(string) -> void = main\n  return 0\n}")
	ls := mainBody(t, prog)[0].(*ast.LetStmt)
	if ls.Type != ast.FuncType([]ast.TypeName{ast.TypeString}, ast.TypeVoid) {
		t.Errorf("type = %q, want fn(string)->void", ls.Type)
	}
}

func TestParseFuncType_NoParams(t *testing.T) {
	prog := parseOK(t, "fn main() -> int {\n  let f: fn() -> int = main\n  return 0\n}")
	ls := mainBody(t, prog)[0].(*ast.LetStmt)
	if ls.Type != ast.FuncType(nil, ast.TypeInt) {
		t.Errorf("type = %q, want fn()->int", ls.Type)
	}
}

func TestParseFuncType_NestedReturn(t *testing.T) {
	// Return type is itself a composite (an array of a function type).
	prog := parseOK(t, "fn main() -> int {\n  let f: fn(int) -> (fn(int) -> bool)[] = main\n  return 0\n}")
	ls := mainBody(t, prog)[0].(*ast.LetStmt)
	want := ast.FuncType([]ast.TypeName{ast.TypeInt}, ast.ArrayType(ast.FuncType([]ast.TypeName{ast.TypeInt}, ast.TypeBool)))
	if ls.Type != want {
		t.Errorf("type = %q, want %q", ls.Type, want)
	}
}

func TestParseFuncType_NestedParam(t *testing.T) {
	// A parameter is itself a function type.
	prog := parseOK(t, "fn main() -> int {\n  let f: fn(fn(int) -> int, int) -> int = main\n  return 0\n}")
	ls := mainBody(t, prog)[0].(*ast.LetStmt)
	want := ast.FuncType([]ast.TypeName{ast.FuncType([]ast.TypeName{ast.TypeInt}, ast.TypeInt), ast.TypeInt}, ast.TypeInt)
	if ls.Type != want {
		t.Errorf("type = %q, want %q", ls.Type, want)
	}
}

func TestParseFuncType_VoidParam_Error(t *testing.T) {
	parseErr(t, "fn main() -> int {\n  let f: fn(void) -> int = main\n  return 0\n}")
}

// --- callee-as-expression: postfix `(...)` on any primary ---

func TestParseCall_BareName(t *testing.T) {
	prog := parseOK(t, wrap(`f(1, 2)`))
	call := mainBody(t, prog)[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	if call.CalleeName != "f" {
		t.Errorf("CalleeName = %q, want f", call.CalleeName)
	}
	if _, ok := call.Callee.(*ast.Ident); !ok {
		t.Errorf("Callee = %T, want *ast.Ident", call.Callee)
	}
}

func TestParseCall_FieldCallee(t *testing.T) {
	prog := parseOK(t, wrap(`s.op(1)`))
	call := mainBody(t, prog)[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	if call.CalleeName != "" {
		t.Errorf("CalleeName = %q, want empty (non-ident callee)", call.CalleeName)
	}
	if _, ok := call.Callee.(*ast.FieldAccess); !ok {
		t.Errorf("Callee = %T, want *ast.FieldAccess", call.Callee)
	}
}

func TestParseCall_IndexCallee(t *testing.T) {
	prog := parseOK(t, wrap(`fns[0](1)`))
	call := mainBody(t, prog)[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	if _, ok := call.Callee.(*ast.IndexExpr); !ok {
		t.Errorf("Callee = %T, want *ast.IndexExpr", call.Callee)
	}
}

func TestParseCall_CallResultCallee(t *testing.T) {
	// getOp()(z): the callee of the outer call is itself a call.
	prog := parseOK(t, wrap(`getOp()(3)`))
	outer := mainBody(t, prog)[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	if _, ok := outer.Callee.(*ast.CallExpr); !ok {
		t.Errorf("Callee = %T, want *ast.CallExpr", outer.Callee)
	}
}
