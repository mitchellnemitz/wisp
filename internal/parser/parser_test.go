package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

func parseOK(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", src, err)
	}
	return prog
}

func parseErr(t *testing.T, src string) error {
	t.Helper()
	_, err := Parse(src, "test.wisp")
	if err == nil {
		t.Fatalf("Parse(%q): expected error, got none", src)
	}
	return err
}

// wrap puts a statement body inside a main function for tests that exercise
// statement/expression parsing.
func wrap(body string) string {
	return "fn main() -> int {\n" + body + "\n}"
}

func mainBody(t *testing.T, prog *ast.Program) []ast.Stmt {
	t.Helper()
	if len(prog.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(prog.Funcs))
	}
	return prog.Funcs[0].Body
}

func TestConversionBuiltinCalls(t *testing.T) {
	cases := []struct{ src, callee string }{
		{`let n: int = to_int("5")`, "to_int"},
		{`let b: bool = to_bool(n)`, "to_bool"},
		{`let s: string = to_string(n)`, "to_string"},
	}
	for _, c := range cases {
		prog := parseOK(t, wrap(c.src))
		body := mainBody(t, prog)
		ls, ok := body[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("%s: body[0] = %T, want *ast.LetStmt", c.src, body[0])
		}
		call, ok := ls.Value.(*ast.CallExpr)
		if !ok {
			t.Fatalf("%s: value = %T, want *ast.CallExpr", c.src, ls.Value)
		}
		if call.CalleeName != c.callee {
			t.Errorf("%s: callee = %q, want %q", c.src, call.CalleeName, c.callee)
		}
	}
}

func TestBareTypeNameNotValue(t *testing.T) {
	parseErr(t, wrap(`let n: int = int`))
}

func TestFuncDecl(t *testing.T) {
	prog := parseOK(t, "fn add(a: int, b: int) -> int {\n  return a + b\n}")
	if len(prog.Funcs) != 1 {
		t.Fatalf("funcs = %d, want 1", len(prog.Funcs))
	}
	f := prog.Funcs[0]
	if f.Name != "add" {
		t.Errorf("name = %q, want add", f.Name)
	}
	if f.RetType != ast.TypeInt {
		t.Errorf("ret = %q, want int", f.RetType)
	}
	if len(f.Params) != 2 || f.Params[0].Name != "a" || f.Params[0].Type != ast.TypeInt ||
		f.Params[1].Name != "b" || f.Params[1].Type != ast.TypeInt {
		t.Fatalf("params = %+v", f.Params)
	}
	if f.Params[0].Default != nil {
		t.Errorf("param a should have no default")
	}
	if len(f.Body) != 1 {
		t.Fatalf("body stmts = %d, want 1", len(f.Body))
	}
	ret, ok := f.Body[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *ReturnStmt", f.Body[0])
	}
	bin, ok := ret.Value.(*ast.BinaryExpr)
	if !ok || bin.Op != token.Plus {
		t.Fatalf("return value = %T, want a+b binary", ret.Value)
	}
	// fn keyword position
	if f.Pos().Line != 1 || f.Pos().Col != 1 {
		t.Errorf("fn pos = %v, want 1:1", f.Pos())
	}
}

func TestVoidFuncAndEmptyReturn(t *testing.T) {
	prog := parseOK(t, "fn greet(name: string) -> void {\n  return\n}")
	f := prog.Funcs[0]
	if f.RetType != ast.TypeVoid {
		t.Errorf("ret = %q, want void", f.RetType)
	}
	ret := f.Body[0].(*ast.ReturnStmt)
	if ret.Value != nil {
		t.Errorf("void return should have nil Value, got %T", ret.Value)
	}
}

func TestParamDefault(t *testing.T) {
	prog := parseOK(t, `fn log(msg: string, prefix: string = "[info] ") -> void {
  return
}`)
	f := prog.Funcs[0]
	if f.Params[0].Default != nil {
		t.Errorf("msg should have no default")
	}
	def := f.Params[1].Default
	if def == nil {
		t.Fatalf("prefix should have a default")
	}
	sl, ok := def.(*ast.StringLit)
	if !ok {
		t.Fatalf("default = %T, want *StringLit", def)
	}
	if len(sl.Parts) != 1 || sl.Parts[0].Text != "[info] " {
		t.Errorf("default parts = %+v", sl.Parts)
	}
}

func TestParamDefaultNegativeIntConst(t *testing.T) {
	// constant expression: unary - applied to an int literal
	prog := parseOK(t, "fn f(n: int = -1) -> void {\n  return\n}")
	def := prog.Funcs[0].Params[0].Default
	u, ok := def.(*ast.UnaryExpr)
	if !ok || u.Op != token.Minus {
		t.Fatalf("default = %T, want unary minus", def)
	}
	if _, ok := u.X.(*ast.IntLit); !ok {
		t.Fatalf("unary operand = %T, want IntLit", u.X)
	}
}

func TestLetStmt(t *testing.T) {
	prog := parseOK(t, wrap("let count: int = 0"))
	stmts := mainBody(t, prog)
	let, ok := stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *LetStmt", stmts[0])
	}
	if let.Name != "count" || let.Type != ast.TypeInt {
		t.Errorf("let = %+v", let)
	}
	if il, ok := let.Value.(*ast.IntLit); !ok || il.Raw != "0" {
		t.Fatalf("value = %T", let.Value)
	}
}

func TestFloatLetStmt(t *testing.T) {
	prog := parseOK(t, wrap("let pi: float = 3.14"))
	stmts := mainBody(t, prog)
	let, ok := stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *LetStmt", stmts[0])
	}
	if let.Name != "pi" || let.Type != ast.TypeFloat {
		t.Errorf("let = %+v", let)
	}
	fl, ok := let.Value.(*ast.FloatLit)
	if !ok || fl.Raw != "3.14" {
		t.Fatalf("value = %T (%v)", let.Value, let.Value)
	}
}

func TestFloatUnaryMinus(t *testing.T) {
	prog := parseOK(t, wrap("let n: float = -2.0"))
	stmts := mainBody(t, prog)
	let := stmts[0].(*ast.LetStmt)
	u, ok := let.Value.(*ast.UnaryExpr)
	if !ok || u.Op != token.Minus {
		t.Fatalf("value = %T, want unary minus", let.Value)
	}
	if fl, ok := u.X.(*ast.FloatLit); !ok || fl.Raw != "2.0" {
		t.Fatalf("unary operand = %T, want FloatLit 2.0", u.X)
	}
}

func TestFloatConversionCall(t *testing.T) {
	prog := parseOK(t, wrap(`let f: float = to_float(2)`))
	stmts := mainBody(t, prog)
	let := stmts[0].(*ast.LetStmt)
	call, ok := let.Value.(*ast.CallExpr)
	if !ok || call.CalleeName != "to_float" {
		t.Fatalf("value = %T, want to_float() call", let.Value)
	}
}

func TestFloatParamAndReturn(t *testing.T) {
	prog := parseOK(t, "fn dbl(x: float) -> float {\n  return x\n}")
	f := prog.Funcs[0]
	if f.RetType != ast.TypeFloat {
		t.Errorf("ret = %q, want float", f.RetType)
	}
	if len(f.Params) != 1 || f.Params[0].Type != ast.TypeFloat {
		t.Fatalf("params = %+v", f.Params)
	}
}

func TestOptionalTypeParsing(t *testing.T) {
	cases := []struct {
		ann  string
		want ast.TypeName
	}{
		{"Optional[int]", "Optional[int]"},
		{"Optional[int[]]", "Optional[[int]]"},
		{"Optional[{string:int}]", "Optional[{string:int}]"},
		{"Optional[Optional[int]]", "Optional[Optional[int]]"},
		{"Optional[Point]", "Optional[Point]"},
	}
	for _, c := range cases {
		prog := parseOK(t, wrap("let a: "+c.ann+" = a"))
		stmts := mainBody(t, prog)
		let, ok := stmts[0].(*ast.LetStmt)
		if !ok {
			t.Fatalf("%s: stmt 0 = %T, want *LetStmt", c.ann, stmts[0])
		}
		if let.Type != c.want {
			t.Errorf("%s: let.Type = %q, want %q", c.ann, let.Type, c.want)
		}
	}
}

func TestParseQualifiedGenericTypeName(t *testing.T) {
	prog := parseOK(t, wrap("let b: bx.Box[int] = 0"))
	stmts := mainBody(t, prog)
	let, ok := stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *LetStmt", stmts[0])
	}
	want := ast.TypeName("bx.Box[int]")
	if let.Type != want {
		t.Errorf("let.Type = %q, want %q", let.Type, want)
	}
}

func TestParseQualifiedArrayPostfixStillWorks(t *testing.T) {
	prog := parseOK(t, wrap("let b: bx.Box[] = []"))
	stmts := mainBody(t, prog)
	let, ok := stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *LetStmt", stmts[0])
	}
	want := ast.TypeName("[bx.Box]")
	if let.Type != want {
		t.Errorf("let.Type = %q, want %q", let.Type, want)
	}
}

func TestParseOptionalWrappingQualifiedGenericTypeName(t *testing.T) {
	prog := parseOK(t, wrap("let b: Optional[bx.Box[int]] = None"))
	stmts := mainBody(t, prog)
	let, ok := stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *LetStmt", stmts[0])
	}
	want := ast.TypeName("Optional[bx.Box[int]]")
	if let.Type != want {
		t.Errorf("let.Type = %q, want %q", let.Type, want)
	}
}

func TestOptionalWildcardElementRejected(t *testing.T) {
	err := parseErr(t, wrap("let a: Optional[?] = a"))
	if !strings.Contains(err.Error(), "?") {
		t.Errorf("error = %q, want mention of '?'", err.Error())
	}
}

func TestAssignStmt(t *testing.T) {
	prog := parseOK(t, wrap("count = count + 1"))
	stmts := mainBody(t, prog)
	as, ok := stmts[0].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *AssignStmt", stmts[0])
	}
	if as.Name != "count" {
		t.Errorf("name = %q", as.Name)
	}
	if _, ok := as.Value.(*ast.BinaryExpr); !ok {
		t.Fatalf("value = %T, want binary", as.Value)
	}
}

func TestExprStmtCall(t *testing.T) {
	prog := parseOK(t, wrap(`print("hi")`))
	stmts := mainBody(t, prog)
	es, ok := stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *ExprStmt", stmts[0])
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || call.CalleeName != "print" {
		t.Fatalf("expr = %T", es.X)
	}
	if len(call.Args) != 1 {
		t.Fatalf("args = %d, want 1", len(call.Args))
	}
}

func TestCallMultiArg(t *testing.T) {
	prog := parseOK(t, wrap("add(1, 2, x)"))
	call := mainBody(t, prog)[0].(*ast.ExprStmt).X.(*ast.CallExpr)
	if len(call.Args) != 3 {
		t.Fatalf("args = %d, want 3", len(call.Args))
	}
}

func TestIfElseIfElse(t *testing.T) {
	src := wrap(`if (x > 10) {
  print("big")
} else if (x > 0) {
  print("small")
} else {
  print("np")
}`)
	stmts := mainBody(t, parseOK(t, src))
	ifs, ok := stmts[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("stmt = %T, want *IfStmt", stmts[0])
	}
	if _, ok := ifs.Cond.(*ast.BinaryExpr); !ok {
		t.Fatalf("cond = %T", ifs.Cond)
	}
	if len(ifs.Then) != 1 {
		t.Errorf("then len = %d", len(ifs.Then))
	}
	if len(ifs.ElseIfs) != 1 {
		t.Fatalf("elseifs = %d, want 1", len(ifs.ElseIfs))
	}
	if len(ifs.ElseIfs[0].Body) != 1 {
		t.Errorf("elseif body = %d", len(ifs.ElseIfs[0].Body))
	}
	if len(ifs.Else) != 1 {
		t.Errorf("else len = %d, want 1", len(ifs.Else))
	}
}

func TestIfNoElse(t *testing.T) {
	stmts := mainBody(t, parseOK(t, wrap("if (true) {\n  return 1\n}")))
	ifs := stmts[0].(*ast.IfStmt)
	if ifs.Else != nil {
		t.Errorf("Else should be nil, got %v", ifs.Else)
	}
	if len(ifs.ElseIfs) != 0 {
		t.Errorf("ElseIfs should be empty")
	}
}

func TestWhile(t *testing.T) {
	stmts := mainBody(t, parseOK(t, wrap("while (i < n) {\n  i = i + 1\n}")))
	w, ok := stmts[0].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("stmt = %T, want *WhileStmt", stmts[0])
	}
	if _, ok := w.Cond.(*ast.BinaryExpr); !ok {
		t.Fatalf("cond = %T", w.Cond)
	}
	if len(w.Body) != 1 {
		t.Errorf("body = %d", len(w.Body))
	}
}

func TestFor(t *testing.T) {
	stmts := mainBody(t, parseOK(t, wrap("for (let i: int = 0; i < n; i = i + 1) {\n  print(\"x\")\n}")))
	f, ok := stmts[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("stmt = %T, want *ForStmt", stmts[0])
	}
	if _, ok := f.Init.(*ast.LetStmt); !ok {
		t.Fatalf("init = %T, want *LetStmt", f.Init)
	}
	if _, ok := f.Cond.(*ast.BinaryExpr); !ok {
		t.Fatalf("cond = %T", f.Cond)
	}
	if _, ok := f.Post.(*ast.AssignStmt); !ok {
		t.Fatalf("post = %T, want *AssignStmt", f.Post)
	}
	if len(f.Body) != 1 {
		t.Errorf("body = %d", len(f.Body))
	}
}

func TestForEmptyClauses(t *testing.T) {
	stmts := mainBody(t, parseOK(t, wrap("for (; true; ) {\n  break\n}")))
	f := stmts[0].(*ast.ForStmt)
	if f.Init != nil {
		t.Errorf("init should be nil")
	}
	if f.Post != nil {
		t.Errorf("post should be nil")
	}
	if _, ok := f.Cond.(*ast.BoolLit); !ok {
		t.Fatalf("cond = %T", f.Cond)
	}
}

func TestSwitch(t *testing.T) {
	src := wrap(`switch (code) {
  case 0 {
    print("ok")
  }
  case 1, 2 {
    print("retry")
  }
  default {
    print("fail")
  }
}`)
	stmts := mainBody(t, parseOK(t, src))
	sw, ok := stmts[0].(*ast.SwitchStmt)
	if !ok {
		t.Fatalf("stmt = %T, want *SwitchStmt", stmts[0])
	}
	if _, ok := sw.Subject.(*ast.Ident); !ok {
		t.Fatalf("subject = %T", sw.Subject)
	}
	if len(sw.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(sw.Cases))
	}
	if len(sw.Cases[0].Values) != 1 {
		t.Errorf("case 0 values = %d, want 1", len(sw.Cases[0].Values))
	}
	if len(sw.Cases[1].Values) != 2 {
		t.Errorf("case 1 values = %d, want 2", len(sw.Cases[1].Values))
	}
	if sw.Default == nil || len(sw.Default) != 1 {
		t.Fatalf("default = %v", sw.Default)
	}
}

func TestSwitchNoDefaultParsesOK(t *testing.T) {
	// parser permits absent default; checker enforces presence
	stmts := mainBody(t, parseOK(t, wrap("switch (x) {\n  case 1 {\n    return 1\n  }\n}")))
	sw := stmts[0].(*ast.SwitchStmt)
	if sw.Default != nil {
		t.Errorf("Default should be nil when absent, got %v", sw.Default)
	}
}

func TestBreakContinue(t *testing.T) {
	stmts := mainBody(t, parseOK(t, wrap("while (true) {\n  break\n  continue\n}")))
	w := stmts[0].(*ast.WhileStmt)
	if _, ok := w.Body[0].(*ast.BreakStmt); !ok {
		t.Errorf("body[0] = %T, want *BreakStmt", w.Body[0])
	}
	if _, ok := w.Body[1].(*ast.ContinueStmt); !ok {
		t.Errorf("body[1] = %T, want *ContinueStmt", w.Body[1])
	}
}

func TestPrecedence(t *testing.T) {
	// 1 + 2 * 3 parses as 1 + (2 * 3)
	prog := parseOK(t, wrap("let r: int = 1 + 2 * 3"))
	v := mainBody(t, prog)[0].(*ast.LetStmt).Value
	add, ok := v.(*ast.BinaryExpr)
	if !ok || add.Op != token.Plus {
		t.Fatalf("top = %T, want + binary", v)
	}
	mul, ok := add.R.(*ast.BinaryExpr)
	if !ok || mul.Op != token.Star {
		t.Fatalf("rhs = %T, want * binary", add.R)
	}
}

func TestPrecedenceFull(t *testing.T) {
	// a || b && c == d  =>  a || (b && (c == d))
	prog := parseOK(t, wrap("let r: bool = a || b && c == d"))
	v := mainBody(t, prog)[0].(*ast.LetStmt).Value
	or := v.(*ast.BinaryExpr)
	if or.Op != token.OrOr {
		t.Fatalf("top op = %v, want ||", or.Op)
	}
	and := or.R.(*ast.BinaryExpr)
	if and.Op != token.AndAnd {
		t.Fatalf("rhs op = %v, want &&", and.Op)
	}
	eq := and.R.(*ast.BinaryExpr)
	if eq.Op != token.Eq {
		t.Fatalf("and rhs op = %v, want ==", eq.Op)
	}
}

func TestUnary(t *testing.T) {
	prog := parseOK(t, wrap("let r: int = -x"))
	v := mainBody(t, prog)[0].(*ast.LetStmt).Value
	u, ok := v.(*ast.UnaryExpr)
	if !ok || u.Op != token.Minus {
		t.Fatalf("v = %T, want unary minus", v)
	}
	prog = parseOK(t, wrap("let r: bool = !ok"))
	v = mainBody(t, prog)[0].(*ast.LetStmt).Value
	u = v.(*ast.UnaryExpr)
	if u.Op != token.Bang {
		t.Fatalf("op = %v, want !", u.Op)
	}
}

func TestUnaryMinusNotPartOfLiteral(t *testing.T) {
	// -5 is unary minus applied to int literal 5
	prog := parseOK(t, wrap("let r: int = -5"))
	v := mainBody(t, prog)[0].(*ast.LetStmt).Value
	u := v.(*ast.UnaryExpr)
	il := u.X.(*ast.IntLit)
	if il.Raw != "5" {
		t.Errorf("literal = %q, want 5 (no sign)", il.Raw)
	}
}

func TestParenGrouping(t *testing.T) {
	// (1 + 2) * 3 parses as (1+2) * 3
	prog := parseOK(t, wrap("let r: int = (1 + 2) * 3"))
	v := mainBody(t, prog)[0].(*ast.LetStmt).Value
	mul := v.(*ast.BinaryExpr)
	if mul.Op != token.Star {
		t.Fatalf("top = %v, want *", mul.Op)
	}
	add := mul.L.(*ast.BinaryExpr)
	if add.Op != token.Plus {
		t.Fatalf("lhs = %v, want +", add.Op)
	}
}

func TestStringLitSingle(t *testing.T) {
	prog := parseOK(t, wrap("let s: string = 'hi'"))
	sl := mainBody(t, prog)[0].(*ast.LetStmt).Value.(*ast.StringLit)
	if len(sl.Parts) != 1 || !sl.Parts[0].IsText() || sl.Parts[0].Text != "hi" {
		t.Fatalf("parts = %+v", sl.Parts)
	}
}

func TestStringLitInterpolated(t *testing.T) {
	prog := parseOK(t, wrap(`let s: string = "count: ${n + 1}!"`))
	sl := mainBody(t, prog)[0].(*ast.LetStmt).Value.(*ast.StringLit)
	if len(sl.Parts) != 3 {
		t.Fatalf("parts = %d, want 3: %+v", len(sl.Parts), sl.Parts)
	}
	if !sl.Parts[0].IsText() || sl.Parts[0].Text != "count: " {
		t.Errorf("part 0 = %+v", sl.Parts[0])
	}
	if sl.Parts[1].IsText() {
		t.Fatalf("part 1 should be an expr")
	}
	if _, ok := sl.Parts[1].Expr.(*ast.BinaryExpr); !ok {
		t.Errorf("part 1 expr = %T, want binary", sl.Parts[1].Expr)
	}
	if !sl.Parts[2].IsText() || sl.Parts[2].Text != "!" {
		t.Errorf("part 2 = %+v", sl.Parts[2])
	}
}

func TestEmptyDoubleString(t *testing.T) {
	prog := parseOK(t, wrap(`let s: string = ""`))
	sl := mainBody(t, prog)[0].(*ast.LetStmt).Value.(*ast.StringLit)
	if len(sl.Parts) != 0 {
		t.Errorf(`"" should have 0 parts, got %+v`, sl.Parts)
	}
}

func TestBoolLit(t *testing.T) {
	prog := parseOK(t, wrap("let a: bool = true\nlet b: bool = false"))
	stmts := mainBody(t, prog)
	if v := stmts[0].(*ast.LetStmt).Value.(*ast.BoolLit); !v.Value {
		t.Errorf("true literal value = false")
	}
	if v := stmts[1].(*ast.LetStmt).Value.(*ast.BoolLit); v.Value {
		t.Errorf("false literal value = true")
	}
}

func TestSemicolonSeparators(t *testing.T) {
	stmts := mainBody(t, parseOK(t, wrap("let a: int = 1; let b: int = 2; a = b")))
	if len(stmts) != 3 {
		t.Fatalf("stmts = %d, want 3", len(stmts))
	}
}

func TestMultipleFuncs(t *testing.T) {
	src := "fn a() -> void {\n  return\n}\nfn b() -> int {\n  return 0\n}"
	prog := parseOK(t, src)
	if len(prog.Funcs) != 2 {
		t.Fatalf("funcs = %d, want 2", len(prog.Funcs))
	}
	if prog.Funcs[0].Name != "a" || prog.Funcs[1].Name != "b" {
		t.Errorf("names = %q, %q", prog.Funcs[0].Name, prog.Funcs[1].Name)
	}
}

// --- parse error cases ---

func errAt(t *testing.T, err error, wantSubstr string) {
	t.Helper()
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("error %q does not contain %q", err.Error(), wantSubstr)
	}
}

func TestErrIfMissingParens(t *testing.T) {
	// `if x > 10 {` — missing parens around condition
	err := parseErr(t, wrap("if x > 10 {\n  return 1\n}"))
	errAt(t, err, "test.wisp:2:") // the 'if' is on line 2 of the wrapped source
}

func TestErrMatchDefaultArm(t *testing.T) {
	// success criterion 3: match's `default` arm (switch's catch-all
	// spelling, not match's) must emit the targeted message, not the
	// generic "expected case, got default".
	err := parseErr(t, wrap(`match (o) {
case Some(_) { print("some") }
default { print("none") }
}`))
	errAt(t, err, `match has no "default" arm; use "case _"`)
}

func TestErrIfBracelessBranch(t *testing.T) {
	// brace-less then-branch
	parseErr(t, wrap("if (x > 10) return 1"))
}

func TestErrWhileMissingParens(t *testing.T) {
	parseErr(t, wrap("while i < n {\n  i = i + 1\n}"))
}

func TestErrMissingBody(t *testing.T) {
	// while with no braces around body
	parseErr(t, wrap("while (true) i = 1"))
}

func TestErrForMissingParens(t *testing.T) {
	parseErr(t, wrap("for let i: int = 0; i < 1; i = i + 1 {\n  break\n}"))
}

func TestErrFuncMissingArrow(t *testing.T) {
	parseErr(t, "fn f() int {\n  return 0\n}")
}

func TestErrFuncMissingBody(t *testing.T) {
	parseErr(t, "fn f() -> int return 0")
}

func TestErrSwitchMissingParens(t *testing.T) {
	parseErr(t, wrap("switch x {\n  default {\n    return 0\n  }\n}"))
}

func TestErrUnterminatedParen(t *testing.T) {
	parseErr(t, wrap("let r: int = (1 + 2"))
}

func TestErrPositionPresent(t *testing.T) {
	err := parseErr(t, "fn f() -> int {\n  let x int = 0\n}") // missing colon
	errAt(t, err, "test.wisp:2:")
}

func TestErrLexErrorPropagates(t *testing.T) {
	// a lex-level error surfaces through Parse with position
	err := parseErr(t, `fn f() -> int {  let s: string = "bad\q"  }`)
	errAt(t, err, "test.wisp:1:")
}

func TestErrTopLevelNotFn(t *testing.T) {
	// only function declarations at top level
	parseErr(t, "let x: int = 0")
}

func TestParseTypeParams(t *testing.T) {
	prog := parseOK(t, "fn id[T](x: T) -> T { return x }\n"+wrap("return 0"))
	got := prog.Funcs[0].TypeParams
	if len(got) != 1 || got[0] != "T" {
		t.Fatalf("TypeParams = %v, want [T]", got)
	}
	prog = parseOK(t, "fn apply[T, U](x: T, f: fn(T) -> U) -> U { return f(x) }\n"+wrap("return 0"))
	if g := prog.Funcs[0].TypeParams; len(g) != 2 || g[0] != "T" || g[1] != "U" {
		t.Fatalf("TypeParams = %v, want [T U]", g)
	}
}

func TestParseTypeParamsErrors(t *testing.T) {
	for _, src := range []string{
		"fn f[]() -> int { return 0 }",               // empty list
		"fn f[T, T](x: T) -> T { return x }",         // duplicate
		"fn f[int](x: int) -> int { return 0 }",      // primitive collision
		"fn f[Optional](x: int) -> int { return 0 }", // Optional collision
		"fn f[string](x: int) -> int { return 0 }",   // primitive collision
	} {
		parseErr(t, src)
	}
}

func TestParseTypeParamBounds(t *testing.T) {
	for _, src := range []string{
		"fn f[T: comparable](a: T) -> bool { return true }\nfn main() -> int { return 0 }",
		"fn g[T: comparable, U: comparable](a: T, b: U) -> bool { return true }\nfn main() -> int { return 0 }",
		"fn h[T: comparable, U](a: T, b: U) -> bool { return true }\nfn main() -> int { return 0 }",
		// comparable as an ordinary type-param name (outside bound position) still parses
		"fn k[comparable](x: comparable) -> comparable { return x }\nfn main() -> int { return 0 }",
	} {
		if _, err := Parse(src, "t.wisp"); err != nil {
			t.Errorf("parse %q: %v", src, err)
		}
	}
}

func TestParseTypeParamBoundUnknown(t *testing.T) {
	_, err := Parse("fn f[T: ordered](a: T) -> bool { return true }\nfn main() -> int { return 0 }", "t.wisp")
	if err == nil || !strings.Contains(err.Error(), "comparable") {
		t.Fatalf("want unknown-bound error naming comparable, got %v", err)
	}
}

// binExpr parses src as the value of a let in main() and returns the top-level BinaryExpr.
func binExpr(t *testing.T, src string) *ast.BinaryExpr {
	t.Helper()
	prog := parseOK(t, wrap("let _r: int = "+src))
	v := mainBody(t, prog)[0].(*ast.LetStmt).Value
	b, ok := v.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("binExpr(%q): top = %T, want *ast.BinaryExpr", src, v)
	}
	return b
}

// assertOp asserts that e is a *ast.BinaryExpr with the given Op.
func assertOp(t *testing.T, e ast.Expr, want token.Kind) {
	t.Helper()
	b, ok := e.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("assertOp: got %T, want *ast.BinaryExpr with op %v", e, want)
	}
	if b.Op != want {
		t.Fatalf("assertOp: op = %v, want %v", b.Op, want)
	}
}

func TestBitwisePrecedence(t *testing.T) {
	// flags & MASK == 0  =>  (flags & MASK) == 0   [bitwise tighter than ==]
	eqE := binExpr(t, "a & b == c")
	assertOp(t, eqE, token.Eq)
	assertOp(t, eqE.L, token.Amp)

	// a | b & c  =>  a | (b & c)   [& tighter than |]
	orE := binExpr(t, "a | b & c")
	assertOp(t, orE, token.Pipe)
	assertOp(t, orE.R, token.Amp)

	// a ^ b | c  =>  (a ^ b) | c   [^ tighter than |]
	xorOr := binExpr(t, "a ^ b | c")
	assertOp(t, xorOr, token.Pipe)
	assertOp(t, xorOr.L, token.Caret)

	// a << b + c  =>  a << (b + c)  [+ tighter than <<]
	sh := binExpr(t, "a << b + c")
	assertOp(t, sh, token.Shl)
	assertOp(t, sh.R, token.Plus)

	// a + b & c  =>  (a + b) & c   [arith tighter than bitwise]
	ab := binExpr(t, "a + b & c")
	assertOp(t, ab, token.Amp)
	assertOp(t, ab.L, token.Plus)

	// a & b << c  =>  a & (b << c)  [shift tighter than &]
	as := binExpr(t, "a & b << c")
	assertOp(t, as, token.Amp)
	assertOp(t, as.R, token.Shl)

	// boundary: a < b & c => a < (b & c)  [& tighter than <]
	lt := binExpr(t, "a < b & c")
	assertOp(t, lt, token.Lt)
	assertOp(t, lt.R, token.Amp)

	// boundary: a && b | c => a && (b | c)  [| tighter than &&]
	aa := binExpr(t, "a && b | c")
	assertOp(t, aa, token.AndAnd)
	assertOp(t, aa.R, token.Pipe)

	// left-assoc: a >> b >> c => (a >> b) >> c
	ls := binExpr(t, "a >> b >> c")
	assertOp(t, ls, token.Shr)
	assertOp(t, ls.L, token.Shr)

	// left-assoc: a & b & c => (a & b) & c
	la := binExpr(t, "a & b & c")
	assertOp(t, la, token.Amp)
	assertOp(t, la.L, token.Amp)

	// left-assoc: a | b | c => (a | b) | c
	lo := binExpr(t, "a | b | c")
	assertOp(t, lo, token.Pipe)
	assertOp(t, lo.L, token.Pipe)
}
