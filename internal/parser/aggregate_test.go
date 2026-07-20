package parser

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

func TestParseStructDecl(t *testing.T) {
	prog := parseOK(t, "struct Point { x: int, y: int }\nfn main() -> int { return 0 }")
	if len(prog.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(prog.Structs))
	}
	sd := prog.Structs[0]
	if sd.Name != "Point" || len(sd.Fields) != 2 {
		t.Fatalf("unexpected struct decl: %+v", sd)
	}
	if sd.Fields[0].Name != "x" || sd.Fields[0].Type != ast.TypeInt {
		t.Errorf("field 0 = %+v", sd.Fields[0])
	}
}

func TestParseStructLitAndFieldAccess(t *testing.T) {
	prog := parseOK(t, "struct P { x: int }\nfn main() -> int { let p: P = P { x: 1 }\n let z: int = p.x\n p.x = 2\n return 0 }")
	body := prog.Funcs[0].Body
	let0 := body[0].(*ast.LetStmt)
	if _, ok := let0.Value.(*ast.StructLit); !ok {
		t.Errorf("expected StructLit value, got %T", let0.Value)
	}
	let1 := body[1].(*ast.LetStmt)
	if fa, ok := let1.Value.(*ast.FieldAccess); !ok || fa.Field != "x" {
		t.Errorf("expected FieldAccess .x, got %T", let1.Value)
	}
	if _, ok := body[2].(*ast.FieldAssignStmt); !ok {
		t.Errorf("expected FieldAssignStmt, got %T", body[2])
	}
}

func TestParseArrayLitIndexAssign(t *testing.T) {
	prog := parseOK(t, wrap("let xs: int[] = [1, 2, 3]\n let a: int = xs[0]\n xs[1] = 9"))
	body := prog.Funcs[0].Body
	let0 := body[0].(*ast.LetStmt)
	al, ok := let0.Value.(*ast.ArrayLit)
	if !ok || len(al.Elems) != 3 {
		t.Fatalf("expected ArrayLit of 3, got %T", let0.Value)
	}
	if let0.Type != ast.ArrayType(ast.TypeInt) {
		t.Errorf("array type = %q, want [int]", let0.Type)
	}
	let1 := body[1].(*ast.LetStmt)
	if _, ok := let1.Value.(*ast.IndexExpr); !ok {
		t.Errorf("expected IndexExpr, got %T", let1.Value)
	}
	if _, ok := body[2].(*ast.IndexAssignStmt); !ok {
		t.Errorf("expected IndexAssignStmt, got %T", body[2])
	}
}

func TestParseForIn(t *testing.T) {
	prog := parseOK(t, wrap("let xs: int[] = [1]\n for (x in xs) { print(to_string(x)) }"))
	body := prog.Funcs[0].Body
	fi, ok := body[1].(*ast.ForInStmt)
	if !ok {
		t.Fatalf("expected ForInStmt, got %T", body[1])
	}
	if fi.Var != "x" {
		t.Errorf("for-in var = %q, want x", fi.Var)
	}
}

func TestParseForInDistinctFromCStyle(t *testing.T) {
	// A C-style for whose init names a variable must still parse as ForStmt.
	prog := parseOK(t, wrap("for (let i: int = 0; i < 3; i = i + 1) { print(to_string(i)) }"))
	if _, ok := prog.Funcs[0].Body[0].(*ast.ForStmt); !ok {
		t.Errorf("expected ForStmt, got %T", prog.Funcs[0].Body[0])
	}
}

func TestParseMainArgsSignature(t *testing.T) {
	prog := parseOK(t, "fn main(args: string[]) -> int { return 0 }")
	p := prog.Funcs[0].Params[0]
	if p.Name != "args" || p.Type != ast.ArrayType(ast.TypeString) {
		t.Errorf("main param = %+v, want args: string[]", p)
	}
}

func TestParseNestedArrayType(t *testing.T) {
	prog := parseOK(t, wrap("let g: int[][] = [[1], [2]]"))
	let0 := prog.Funcs[0].Body[0].(*ast.LetStmt)
	if let0.Type != ast.ArrayType(ast.ArrayType(ast.TypeInt)) {
		t.Errorf("type = %q, want int[][]", let0.Type)
	}
}

func TestParseDictTypeAndLit(t *testing.T) {
	prog := parseOK(t, wrap("let m: {string: int} = { \"a\": 1, \"b\": 2 }"))
	let0 := prog.Funcs[0].Body[0].(*ast.LetStmt)
	if let0.Type != ast.DictType(ast.TypeString, ast.TypeInt) {
		t.Errorf("dict type = %q, want {string:int}", let0.Type)
	}
	dl, ok := let0.Value.(*ast.DictLit)
	if !ok || len(dl.Entries) != 2 {
		t.Fatalf("expected DictLit of 2, got %T", let0.Value)
	}
}

func TestParseEmptyDictLit(t *testing.T) {
	prog := parseOK(t, wrap("let m: {int: string} = {}"))
	let0 := prog.Funcs[0].Body[0].(*ast.LetStmt)
	dl, ok := let0.Value.(*ast.DictLit)
	if !ok || len(dl.Entries) != 0 {
		t.Fatalf("expected empty DictLit, got %T", let0.Value)
	}
}

func TestParseNestedDictValueType(t *testing.T) {
	prog := parseOK(t, wrap("let m: {string: int[]} = { \"a\": [1] }"))
	let0 := prog.Funcs[0].Body[0].(*ast.LetStmt)
	if let0.Type != ast.DictType(ast.TypeString, ast.ArrayType(ast.TypeInt)) {
		t.Errorf("type = %q, want {string: int[]}", let0.Type)
	}
}

func TestParseDictLookupAndAssign(t *testing.T) {
	prog := parseOK(t, wrap("let m: {string: int} = {}\n let v: int = m[\"a\"]\n m[\"b\"] = 2"))
	body := prog.Funcs[0].Body
	if _, ok := body[1].(*ast.LetStmt).Value.(*ast.IndexExpr); !ok {
		t.Errorf("expected IndexExpr lookup, got %T", body[1].(*ast.LetStmt).Value)
	}
	if _, ok := body[2].(*ast.IndexAssignStmt); !ok {
		t.Errorf("expected IndexAssignStmt, got %T", body[2])
	}
}

func TestParseChainedPostfix(t *testing.T) {
	// a[0].field[1] must chain index/field/index left-to-right.
	prog := parseOK(t, "struct P { xs: int[] }\nfn main() -> int { let ps: P[] = [P { xs: [1] }]\n let v: int = ps[0].xs[0]\n return 0 }")
	let1 := prog.Funcs[0].Body[1].(*ast.LetStmt)
	idx, ok := let1.Value.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("outer not IndexExpr: %T", let1.Value)
	}
	if _, ok := idx.X.(*ast.FieldAccess); !ok {
		t.Errorf("expected FieldAccess under index, got %T", idx.X)
	}
}
