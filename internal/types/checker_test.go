package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// check parses src and runs the checker, failing the test on a parse error.
func check(t *testing.T, src string) *Info {
	t.Helper()
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v\nsrc:\n%s", err, src)
	}
	return Check(prog)
}

// wrapMain puts a body inside a valid `fn main() -> int { ... return 0 }`.
func wrapMain(body string) string {
	return "fn main() -> int {\n" + body + "\nreturn 0\n}"
}

// expectOK asserts the program type-checks with no errors (warnings allowed).
func expectOK(t *testing.T, src string) *Info {
	t.Helper()
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s\nsrc:\n%s", diagList(info.Errors), src)
	}
	return info
}

// expectErr asserts at least one error whose message contains want, and returns
// the matching diagnostic for position assertions.
func expectErr(t *testing.T, src, want string) Diagnostic {
	t.Helper()
	info := check(t, src)
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, want) {
			return d
		}
	}
	t.Fatalf("expected an error containing %q, got:\n%s\nsrc:\n%s", want, diagList(info.Errors), src)
	return Diagnostic{}
}

func diagList(ds []Diagnostic) string {
	if len(ds) == 0 {
		return "  (none)"
	}
	var b strings.Builder
	for _, d := range ds {
		b.WriteString("  " + d.String() + "\n")
	}
	return b.String()
}

// --- Rule 1: no implicit coercion ---

func TestRule1_NoCoercion_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let a: int = 1
let b: int = 2
let c: bool = a == b`))
}

func TestRule1_NoCoercion_Negative(t *testing.T) {
	d := expectErr(t, wrapMain(`let c: bool = 1 == "1"`), "same type")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
}

// --- Rule 2: conditions must be bool ---

func TestRule2_Condition_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let n: int = 3
if (n > 0) { print("pos") }`))
}

func TestRule2_Condition_Negative(t *testing.T) {
	expectErr(t, wrapMain(`if (1) { print("x") }`), "condition must be bool")
}

func TestRule2_WhileCondition_Negative(t *testing.T) {
	expectErr(t, wrapMain(`while ("x") { break }`), "condition must be bool")
}

// --- Rule 3: let requires annotation + initializer + matching type ---

func TestRule3_Let_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let s: string = "hi"
print(s)`))
}

func TestRule3_Let_TypeMismatch_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let s: string = 1`), "want string")
}

func TestRule3_Assign_TypeMismatch_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 0
n = "x"`), "cannot assign")
}

func TestRule3_Assign_Undeclared_Negative(t *testing.T) {
	expectErr(t, wrapMain(`x = 1`), "undeclared variable")
}

// --- Rule 4: non-void functions must return on every path ---

func TestRule4_AllPathsReturn_Positive(t *testing.T) {
	expectOK(t, `fn pick(n: int) -> int {
  if (n > 0) { return 1 } else { return 2 }
}
fn main() -> int { return pick(1) }`)
}

func TestRule4_MissingReturnPath_Negative(t *testing.T) {
	expectErr(t, `fn pick(n: int) -> int {
  if (n > 0) { return 1 }
}
fn main() -> int { return 0 }`, "must return")
}

func TestRule4_VoidNoReturn_Positive(t *testing.T) {
	expectOK(t, `fn greet() -> void { print("hi") }
fn main() -> int { greet(); return 0 }`)
}

func TestRule4_ReturnTypeMismatch_Negative(t *testing.T) {
	expectErr(t, `fn f() -> int { return "x" }
fn main() -> int { return 0 }`, "return value has type string")
}

func TestRule4_ValueInVoid_Negative(t *testing.T) {
	expectErr(t, `fn f() -> void { return 1 }
fn main() -> int { return 0 }`, "value in a void function")
}

// --- Rule 5: switch requires default ---

func TestRule5_SwitchDefault_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let n: int = 0
switch (n) {
  case 0 { print("zero") }
  default {}
}`))
}

func TestRule5_SwitchNoDefault_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 0
switch (n) {
  case 0 { print("zero") }
}`), "must have a default")
}

// --- Rule 6: unused local is a WARNING, not an error ---

func TestRule6_UnusedLocal_IsWarning(t *testing.T) {
	info := check(t, wrapMain(`let unusedVar: int = 5`))
	if len(info.Errors) != 0 {
		t.Fatalf("unused local must not be an error, got: %s", diagList(info.Errors))
	}
	found := false
	for _, w := range info.Warnings {
		if strings.Contains(w.Msg, "unused variable") && strings.Contains(w.Msg, "unusedVar") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unused-variable warning, got: %s", diagList(info.Warnings))
	}
}

func TestRule6_UsedLocal_NoWarning(t *testing.T) {
	info := expectOK(t, wrapMain(`let n: int = 5
let m: int = n + 1
print("${m}")`))
	if len(info.Warnings) != 0 {
		t.Fatalf("expected no warnings, got: %s", diagList(info.Warnings))
	}
}

// --- Rule 7: + overload ---

func TestRule7_IntPlusInt_Positive(t *testing.T) {
	info := expectOK(t, wrapMain(`let n: int = 1 + 2`))
	_ = info
}

func TestRule7_StringConcat_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let s: string = "a" + "b"`))
}

func TestRule7_MixedPlus_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let s: string = "a" + 1`), "int+int, float+float, or string+string")
}

func TestRule7_MinusStringOnly_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let s: string = "a" - "b"`), "requires int+int or float+float")
}

func TestRule7_LessThanString_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let b: bool = "a" < "b"`), "requires int+int or float+float operands")
}

func TestRule7_EqBool_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let b: bool = true == false`))
}

func TestRule7_EqString_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let b: bool = "a" == "b"`))
}

// --- Rule 8: switch subject int/string only ---

func TestRule8_SwitchInt_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let n: int = 1
switch (n) { case 1 { print("one") } default {} }`))
}

func TestRule8_SwitchString_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let s: string = "a"
switch (s) { case "a" { print("a") } default {} }`))
}

func TestRule8_SwitchBool_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let b: bool = true
switch (b) { case true { print("t") } default {} }`), "subject must be int or string")
}

func TestRule8_CaseTypeMismatch_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 1
switch (n) { case "a" { print("x") } default {} }`), "case value has type string")
}

func TestRule8_CaseNonLiteral_Negative(t *testing.T) {
	// Variable reference in a case value is rejected by checkConstExpr.
	expectErr(t, wrapMain(`let n: int = 1
let m: int = 2
switch (n) { case m { print("x") } default {} }`), "constant expression")
}

// --- Rule 9: print's `to` must be stdout/stderr ---

func TestRule9_PrintStdout_Positive(t *testing.T) {
	expectOK(t, wrapMain(`print("x", stdout)`))
}

func TestRule9_PrintStderr_Positive(t *testing.T) {
	expectOK(t, wrapMain(`print("x", stderr)`))
}

func TestRule9_PrintArbitraryInt_Negative(t *testing.T) {
	expectErr(t, wrapMain(`print("x", 1)`), "must be the constant stdout or stderr")
}

// --- Rule 11: lexical block scope, no redeclaration/shadowing ---

func TestRule11_Redeclaration_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let x: int = 1
let x: int = 2`), "already declared")
}

func TestRule11_Shadowing_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let x: int = 1
if (true) { let x: int = 2; print("${x}") }`), "already declared")
}

func TestRule11_ParamShadow_Negative(t *testing.T) {
	expectErr(t, `fn f(x: int) -> int { let x: int = 2; return x }
fn main() -> int { return f(1) }`, "already declared")
}

func TestRule11_SiblingScopeReuse_Accepted(t *testing.T) {
	expectOK(t, wrapMain(`for (let i: int = 0; i < 2; i = i + 1) { print("${i}") }
for (let i: int = 0; i < 2; i = i + 1) { print("${i}") }`))
}

func TestRule11_SiblingIfArms_Accepted(t *testing.T) {
	expectOK(t, wrapMain(`if (true) { let x: int = 1; print("${x}") } else { let x: int = 2; print("${x}") }`))
}

func TestRule11_SiblingMangledDistinct(t *testing.T) {
	info := expectOK(t, wrapMain(`for (let i: int = 0; i < 2; i = i + 1) { print("${i}") }
for (let i: int = 0; i < 2; i = i + 1) { print("${i}") }`))
	fi := info.Funcs[info.Main]
	var iVars []*Var
	for _, v := range fi.Decls {
		if v.Name == "i" {
			iVars = append(iVars, v)
		}
	}
	if len(iVars) != 2 {
		t.Fatalf("expected 2 distinct `i` declarations, got %d", len(iVars))
	}
	if iVars[0].Mangled == iVars[1].Mangled {
		t.Errorf("sibling-scope `i` declarations share a mangled name %q", iVars[0].Mangled)
	}
}

// --- Rule 12: break/continue require an enclosing loop ---

func TestRule12_BreakInLoop_Positive(t *testing.T) {
	expectOK(t, wrapMain(`while (true) { break }`))
}

func TestRule12_BreakOutsideLoop_Negative(t *testing.T) {
	expectErr(t, wrapMain(`break`), "break outside a loop")
}

func TestRule12_ContinueOutsideLoop_Negative(t *testing.T) {
	expectErr(t, wrapMain(`continue`), "continue outside a loop")
}

func TestRule12_ContinueInSwitchOnly_Negative(t *testing.T) {
	// switch is not a loop; break/continue inside it (with no enclosing loop) error.
	expectErr(t, wrapMain(`let n: int = 0
switch (n) { case 0 { continue } default {} }`), "continue outside a loop")
}

// --- Reserved identifiers ---

func TestReserved_DoubleUnderscoreVar_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let __x: int = 1`), "reserved")
}

func TestReserved_DoubleUnderscoreFunc_Negative(t *testing.T) {
	expectErr(t, `fn __helper() -> int { return 0 }
fn main() -> int { return 0 }`, "reserved")
}

func TestReserved_RedefineBuiltin_Negative(t *testing.T) {
	expectErr(t, `fn print() -> void {}
fn main() -> int { return 0 }`, "reserved builtin")
}

func TestReserved_RedefineStdout_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let stdout: int = 5`), "reserved builtin or constant")
}

func TestReserved_RedefineStderrParam_Negative(t *testing.T) {
	expectErr(t, `fn f(stderr: int) -> int { return stderr }
fn main() -> int { return f(2) }`, "reserved")
}

// --- main signature ---

func TestMain_Missing_Negative(t *testing.T) {
	expectErr(t, `fn f() -> int { return 0 }`, "no main function")
}

func TestMain_Multiple_Negative(t *testing.T) {
	expectErr(t, `fn main() -> int { return 0 }
fn main() -> int { return 1 }`, "declared more than once")
}

func TestMain_WrongReturn_Negative(t *testing.T) {
	expectErr(t, `fn main() -> void { print("x") }`, "must return int")
}

func TestMain_WithParams_Negative(t *testing.T) {
	// A single non-string[] parameter on main is rejected (only args: string[] is
	// permitted, M3 spec 4.5).
	expectErr(t, `fn main(x: int) -> int { return x }`, "string[]")
}

func TestMain_TwoParams_Negative(t *testing.T) {
	expectErr(t, `fn main(a: string[], b: int) -> int { return 0 }`, "either no parameters or a single")
}

func TestMain_OK(t *testing.T) {
	info := expectOK(t, `fn main() -> int { return 0 }`)
	if info.Main == nil {
		t.Fatal("Main not set for a valid program")
	}
	if info.Funcs[info.Main].Mangled != "__wisp_f_m0_main" {
		t.Errorf("main mangled name = %q, want __wisp_f_m0_main", info.Funcs[info.Main].Mangled)
	}
}

// --- Default arguments (spec 10.3) ---

func TestDefault_TrailingFilled_Positive(t *testing.T) {
	info := expectOK(t, `fn greet(name: string, prefix: string = "[info] ") -> void {
  print(prefix + name)
}
fn main() -> int { greet("world"); return 0 }`)
	// find the call greet("world") and assert default filled.
	var ci *CallInfo
	for _, c := range info.Calls {
		if c.Kind == CallUser && c.Func.Name == "greet" {
			ci = c
		}
	}
	if ci == nil {
		t.Fatal("greet call not resolved")
	}
	if len(ci.Args) != 2 {
		t.Fatalf("expected 2 filled args, got %d", len(ci.Args))
	}
	if _, ok := ci.Args[1].(*ast.StringLit); !ok {
		t.Errorf("filled default arg is %T, want *ast.StringLit", ci.Args[1])
	}
}

func TestDefault_NonConst_Negative(t *testing.T) {
	expectErr(t, `fn f(a: int, b: int = a) -> int { return a + b }
fn main() -> int { return f(1) }`, "constant expression may not reference a variable")
}

func TestDefault_OnNonTrailing_Negative(t *testing.T) {
	expectErr(t, `fn f(a: int = 1, b: int) -> int { return a + b }
fn main() -> int { return f(1, 2) }`, "only trailing parameters may have defaults")
}

func TestDefault_WrongType_Negative(t *testing.T) {
	expectErr(t, `fn f(a: int, b: string = 5) -> int { return a }
fn main() -> int { return f(1) }`, "default for parameter")
}

func TestCall_SkipMiddleArg_Negative(t *testing.T) {
	// f has no defaults; calling with one arg is below the required arity.
	expectErr(t, `fn f(a: int, b: int) -> int { return a + b }
fn main() -> int { return f(1) }`, "expects")
}

// --- Builtins type-check against their signatures ---
//
// NOTE: the type-conversion builtins int/string/bool share names with the type
// keywords, and the current parser (T3) does not yet parse `int(...)` etc. as
// calls. Those builtins are exercised here with hand-built AST so the checker's
// builtin handling is covered independent of that parser gap; builtins whose
// names are plain identifiers (length, replace, lower, ...) go through the
// parser normally.

func TestBuiltin_LengthResult(t *testing.T) {
	expectOK(t, wrapMain(`let n: int = length("hello")`))
}

// replace is removable (string.replace); the arity check migrated to the
// namespaced form (helper wantNsErr lives in core_collections_neg_test.go).
func TestBuiltin_ReplaceArity_Negative(t *testing.T) {
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.replace("a", "b"); return 0 }`, "expects")
}

// callMain builds `fn main() -> int { let _x: <letType> = <call>; return 0 }`
// directly (bypassing the parser's type-keyword-call gap) and checks it.
func checkBuiltinConv(t *testing.T, builtin string, arg ast.Expr, letType ast.TypeName) *Info {
	t.Helper()
	call := &ast.CallExpr{Callee: &ast.Ident{Name: builtin}, CalleeName: builtin, Args: []ast.Expr{arg}}
	let := &ast.LetStmt{Name: "v", Type: letType, Value: call}
	use := &ast.ExprStmt{X: &ast.CallExpr{Callee: &ast.Ident{Name: "print"}, CalleeName: "print", Args: []ast.Expr{
		&ast.StringLit{Parts: []ast.StringPart{{Expr: &ast.Ident{Name: "v"}}}},
	}}}
	ret := &ast.ReturnStmt{Value: &ast.IntLit{Raw: "0"}}
	main := &ast.FuncDecl{Name: "main", RetType: ast.TypeInt, Body: []ast.Stmt{let, use, ret}}
	return Check(&ast.Program{Funcs: []*ast.FuncDecl{main}})
}

func TestBuiltin_IntArgType_Negative(t *testing.T) {
	// to_int(5): argument must be string, not int.
	info := checkBuiltinConv(t, "to_int", &ast.IntLit{Raw: "5"}, ast.TypeInt)
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "want string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected to_int()-arg type error, got: %s", diagList(info.Errors))
	}
}

func TestBuiltin_IntResultIsInt_Positive(t *testing.T) {
	info := checkBuiltinConv(t, "to_int", &ast.StringLit{Parts: []ast.StringPart{{Text: "5"}}}, ast.TypeInt)
	if len(info.Errors) != 0 {
		t.Fatalf("to_int(\"5\") should type-check, got: %s", diagList(info.Errors))
	}
}

func TestBuiltin_StringUnionInput_Positive(t *testing.T) {
	// to_string() accepts int, bool, and string.
	for _, arg := range []ast.Expr{
		&ast.IntLit{Raw: "1"},
		&ast.BoolLit{Value: true},
		&ast.StringLit{Parts: []ast.StringPart{{Text: "x"}}},
	} {
		info := checkBuiltinConv(t, "to_string", arg, ast.TypeString)
		if len(info.Errors) != 0 {
			t.Fatalf("to_string(%T) should type-check, got: %s", arg, diagList(info.Errors))
		}
	}
}

func TestBuiltin_BoolRejectsBoolInput_Negative(t *testing.T) {
	// to_bool() accepts int|float|string, not bool.
	info := checkBuiltinConv(t, "to_bool", &ast.BoolLit{Value: true}, ast.TypeBool)
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "want int|float|string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected bool()-arg type error, got: %s", diagList(info.Errors))
	}
}

func TestBuiltin_BareConversionSpellingsRejected(t *testing.T) {
	// The pre-rename spellings string()/int()/float()/bool() are no longer
	// builtins; each must fail as an undeclared function, not silently resolve.
	for _, call := range []string{
		`to_string(1)`, // control: renamed spelling still resolves
	} {
		expectOK(t, wrapMain("let s: string = "+call))
	}
	for name, call := range map[string]string{
		"string": `let s: string = string(1)`,
		"int":    `let i: int = int("1")`,
		"float":  `let f: float = float(1)`,
		"bool":   `let b: bool = bool("x")`,
	} {
		t.Run(name, func(t *testing.T) {
			expectErr(t, wrapMain(call), "undeclared function")
		})
	}
}

// --- Info: types and uses recorded ---

func TestInfo_RecordsExprTypes(t *testing.T) {
	info := expectOK(t, wrapMain(`let n: int = 1 + 2`))
	// the let value binary expr should be typed int.
	found := false
	for e, ty := range info.Types {
		if _, ok := e.(*ast.BinaryExpr); ok && ty == Int {
			found = true
		}
	}
	if !found {
		t.Error("expected the binary expr to be recorded as int")
	}
}

func TestInfo_RecordsUses(t *testing.T) {
	info := expectOK(t, wrapMain(`let n: int = 1
print("${n}")`))
	if len(info.Uses) == 0 {
		t.Fatal("expected at least one recorded identifier use")
	}
	for _, v := range info.Uses {
		if v.Name == "n" && v.Type != Int {
			t.Errorf("use of n resolved to type %s, want int", v.Type)
		}
	}
}

func TestInfo_CallBuiltinRecorded(t *testing.T) {
	info := expectOK(t, wrapMain(`print("x")`))
	var ci *CallInfo
	for _, c := range info.Calls {
		if c.Kind == CallBuiltin && c.Builtin == "print" {
			ci = c
		}
	}
	if ci == nil {
		t.Fatal("print call not recorded as builtin")
	}
	// `to` default should be filled with stdout.
	if len(ci.Args) != 2 {
		t.Fatalf("print call args = %d, want 2 (default filled)", len(ci.Args))
	}
	id, ok := ci.Args[1].(*ast.Ident)
	if !ok || id.Name != "stdout" {
		t.Errorf("default `to` arg = %v, want stdout ident", ci.Args[1])
	}
}

// --- debug builtin ---

func TestDebug_Int_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let _ : string = debug(42)`))
}

func TestDebug_Float_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let _ : string = debug(1.5)`))
}

func TestDebug_Bool_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let _ : string = debug(true)`))
}

func TestDebug_String_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let _ : string = debug("hello")`))
}

func TestDebug_Array_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let xs: int[] = [1, 2, 3]
let _ : string = debug(xs)`))
}

func TestDebug_Optional_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
let _ : string = debug(o)`))
}

func TestDebug_Result_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let r: Result[int] = Ok(1)
let _ : string = debug(r)`))
}

func TestDebug_Dict_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let d: {string:int} = {"a": 1}
let _ : string = debug(d)`))
}

func TestDebug_Struct_Positive(t *testing.T) {
	expectOK(t, `struct Pt { x: int }
fn main() -> int {
  let p: Pt = Pt { x: 1 }
  let _ : string = debug(p)
  return 0
}`)
}

// debugErrContains asserts info has at least one error whose message
// contains want. Process and json.Value values require module-linked
// checking (checkProcessProg/checkJSONProg below), which returns an *Info
// directly rather than going through expectErr's plain, unlinked Check --
// plain Check has no ast.ImportDecl handling at all, so a bare
// `import "process"` string in source is inert and process.spawn/json.null
// never resolve (confirmed during implementation: this failed with
// "undeclared name" errors, not a debug() diagnostic, before this fix).
func debugErrContains(t *testing.T, info *Info, want string) {
	t.Helper()
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, want) {
			return
		}
	}
	t.Fatalf("expected error containing %q, got:\n%s", want, errMsgs(info))
}

// TestDebug_Process_Negative pins that a bare Process argument -- previously
// entirely unchecked by the checker -- is now rejected at compile time
// instead of reaching genDebugStruct's panic at codegen.go:224 (info.Structs
// has no entry for "Process", a plain string type constant, not a struct).
func TestDebug_Process_Negative(t *testing.T) {
	info := checkProcessProg(t, `fn main() -> int {
  let p: Process = process.spawn(["true"])
  let _ : string = debug(p)
  return 0
}`)
	debugErrContains(t, info, "debug() cannot render")
}

// TestDebug_ProcessArray_Negative pins that Process nested inside a
// composite (not just bare) is also rejected: bare Process is caught by the
// top-level isProcessType case on the argument's own type, but nested
// Process is only caught by debugRenderable's recursive descent into the
// array element type -- a distinct code path from the bare case.
func TestDebug_ProcessArray_Negative(t *testing.T) {
	info := checkProcessProg(t, `fn main() -> int {
  let p: Process = process.spawn(["true"])
  let ps: Process[] = [p]
  let _ : string = debug(ps)
  return 0
}`)
	debugErrContains(t, info, "debug() cannot render")
}

// TestDebug_EnumArray_Negative pins that an enum nested inside an array is
// rejected: today's checkDebugCall only checks c.isEnumType on the argument's
// own top-level type, so Color[] (an array, not an enum) sails through the
// existing check and panics in genDebugStruct's array-element recursion.
func TestDebug_EnumArray_Negative(t *testing.T) {
	expectErr(t, `enum Color { Red, Green, Blue }
fn main() -> int {
  let xs: Color[] = [Color.Red]
  let _ : string = debug(xs)
  return 0
}`, "debug() cannot render")
}

// TestDebug_OptionalEnum_Negative pins the same nested-enum rejection through
// debugRenderable's isOptional branch.
func TestDebug_OptionalEnum_Negative(t *testing.T) {
	expectErr(t, `enum Color { Red, Green, Blue }
fn main() -> int {
  let o: Optional[Color] = Some(Color.Red)
  let _ : string = debug(o)
  return 0
}`, "debug() cannot render")
}

// TestDebug_ResultEnum_Negative pins the same nested-enum rejection through
// debugRenderable's isResult branch -- previously untested for rejection
// (round-2 adversarial review finding).
func TestDebug_ResultEnum_Negative(t *testing.T) {
	expectErr(t, `enum Color { Red, Green, Blue }
fn main() -> int {
  let r: Result[Color] = Ok(Color.Red)
  let _ : string = debug(r)
  return 0
}`, "debug() cannot render")
}

// TestDebug_DictEnum_Negative pins the same nested-enum rejection through
// debugRenderable's isDict branch (value side only; dict keys are always int
// or string per the existing dict-key restriction) -- previously untested
// for rejection (round-2 adversarial review finding).
func TestDebug_DictEnum_Negative(t *testing.T) {
	expectErr(t, `enum Color { Red, Green, Blue }
fn main() -> int {
  let d: {string: Color} = {"a": Color.Red}
  let _ : string = debug(d)
  return 0
}`, "debug() cannot render")
}

// TestDebug_TupleEnum_Negative pins the same nested-enum rejection through
// debugRenderable's isTuple branch -- previously untested for rejection
// (round-2 adversarial review finding).
func TestDebug_TupleEnum_Negative(t *testing.T) {
	expectErr(t, `enum Color { Red, Green, Blue }
fn main() -> int {
  let tp: (Color, int) = (Color.Red, 1)
  let _ : string = debug(tp)
  return 0
}`, "debug() cannot render")
}

// TestDebug_StructFieldEnum_Negative pins the same nested-enum rejection
// through debugRenderable's non-cyclic isStructType field-traversal branch --
// previously untested for rejection (round-2 adversarial review finding).
func TestDebug_StructFieldEnum_Negative(t *testing.T) {
	expectErr(t, `enum Color { Red, Green, Blue }
struct Bad { c: Color }
fn main() -> int {
  let s: Bad = Bad { c: Color.Red }
  let _ : string = debug(s)
  return 0
}`, "debug() cannot render")
}

// TestDebug_SelfReferentialStruct_Negative pins the cycle guard: a
// self-referential struct type-checks today (structs are reference handles,
// not inline value types -- AGENTS.md), so debugRenderable must reject the
// cycle rather than recurse unboundedly in the checker itself.
//
// The None literal is assigned to an explicitly-typed local first, then used
// as the struct-literal field value, rather than written inline as
// `Node { next: None }`: a bare `None` in a struct-literal field position
// does not get expected-type propagation the way a var-decl target does
// (confirmed during implementation -- `let n2: Optional[Node] = None` then
// `Node { next: n2 }` type-checks; `Node { next: None }` directly does not,
// failing with "none requires an expected Optional type here" before ever
// reaching debug()). This is a pre-existing, separate checker gap in
// struct-literal field type propagation, out of scope for this task.
func TestDebug_SelfReferentialStruct_Negative(t *testing.T) {
	expectErr(t, `struct Node { next: Optional[Node] }
fn main() -> int {
  let empty: Optional[Node] = None
  let n: Node = Node { next: empty }
  let _ : string = debug(n)
  return 0
}`, "debug() cannot render")
}

// TestDebug_MutuallyRecursiveStruct_Negative pins the cycle guard against a
// two-hop cycle (A -> B -> A), not just the direct self-reference covered by
// TestDebug_SelfReferentialStruct_Negative -- visiting must catch re-entry
// into A even when B sits in between.
func TestDebug_MutuallyRecursiveStruct_Negative(t *testing.T) {
	expectErr(t, `struct A { b: Optional[B] }
struct B { a: Optional[A] }
fn main() -> int {
  let emptyB: Optional[B] = None
  let a: A = A { b: emptyB }
  let _ : string = debug(a)
  return 0
}`, "debug() cannot render")
}

// TestDebug_DiamondStruct_Positive pins that a non-recursive "diamond" shape
// (two sibling fields of the same struct type) is NOT wrongly rejected as a
// cycle: visiting must track only the active descent path (delete on exit),
// not every struct type ever seen during the walk.
func TestDebug_DiamondStruct_Positive(t *testing.T) {
	expectOK(t, `struct Point { x: int, y: int }
struct Pair { a: Point, b: Point }
fn main() -> int {
  let pr: Pair = Pair { a: Point { x: 1, y: 2 }, b: Point { x: 3, y: 4 } }
  let _ : string = debug(pr)
  return 0
}`)
}

// TestDebug_TypeVarArray_Negative pins that a type variable nested inside a
// composite (not just bare, which TestDebug_TypeVar_Negative already covers)
// is rejected through debugRenderable's isArray branch.
func TestDebug_TypeVarArray_Negative(t *testing.T) {
	expectErr(t, "fn show[T](xs: T[]) -> string { return debug(xs) }\n"+wrapMain("return 0"), "generic type")
}

// TestDebug_ErrorType_Positive pins a required positive control for the
// opaque ErrorType leaf branch (round-3 adversarial review finding: this
// leaf was cross-referenced against genDebugValue but had no positive test).
// Duplicate of TestDebug_Error_Positive's coverage is intentional: this one
// lives in the new-tests block added by this task, grouped with the other
// round-2/round-3 additions rather than mixed into the pre-existing block.
func TestDebug_ErrorType_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("oops")
let _ : string = debug(e)`))
}

// TestDebug_RunResultType_Positive pins a required positive control for the
// opaque RunResult leaf branch (round-3 adversarial review finding).
func TestDebug_RunResultType_Positive(t *testing.T) {
	expectOK(t, `fn use(r: RunResult) -> int {
  let _ : string = debug(r)
  return 0
}
fn main() -> int { return 0 }`)
}

// TestDebug_JSONValuePositive pins a required positive control for the
// opaque jsonValueType leaf branch -- the third of the three round-3
// adversarial review leaves (ErrorType, RunResult, jsonValueType), all cited
// by name in the design spec's Test plan item 1 but only two of which were
// present in an earlier draft of this task (plan-gate round-1 finding,
// corroborated by Correctness and Coverage). Uses checkJSONProg (module-
// linked check), like the Process tests above -- json.null() is a stdlib
// namespace member that plain, unlinked Check cannot resolve.
func TestDebug_JSONValuePositive(t *testing.T) {
	info := checkJSONProg(t, `fn main() -> int {
  let v: json.Value = json.null()
  let _ : string = debug(v)
  return 0
}`)
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s", errMsgs(info))
	}
}

func TestDebug_Tuple_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let tp: (int, string) = (1, "a")
let _ : string = debug(tp)`))
}

func TestDebug_Error_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("oops")
let _ : string = debug(e)`))
}

func TestDebug_Funcref_Positive(t *testing.T) {
	expectOK(t, `fn inc(x: int) -> int { return x + 1 }
fn main() -> int {
  let f: fn(int)->int = inc
  let _ : string = debug(f)
  return 0
}`)
}

func TestDebug_RunResult_Positive(t *testing.T) {
	// debug accepts a RunResult value. The RunResult is taken as a function
	// parameter since the bare run_full spelling that once produced one is gone
	// (it lives at process.run_full now; see core_process_test.go).
	expectOK(t, `fn use(r: RunResult) -> int {
  let _ : string = debug(r)
  return 0
}
fn main() -> int { return 0 }`)
}

func TestDebug_Arity0_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let _ : string = debug()`), "debug expects 1 argument, got 0")
}

func TestDebug_Arity2_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let _ : string = debug(1, 2)`), "debug expects 1 argument, got 2")
}

func TestDebug_VoidArg_Negative(t *testing.T) {
	expectErr(t, `fn noop() -> void {}
fn main() -> int {
  let _ : string = debug(noop())
  return 0
}`, "debug requires a value; got void")
}

func TestDebug_ReservedFn_Negative(t *testing.T) {
	expectErr(t, `fn debug() -> void {}
fn main() -> int { return 0 }`, "reserved builtin or constant name")
}

func TestDebug_ReservedLet_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let debug: int = 1`), "reserved builtin or constant name")
}

func TestDebug_ReservedParam_Negative(t *testing.T) {
	expectErr(t, `fn f(debug: int) -> int { return debug }
fn main() -> int { return f(1) }`, "reserved builtin or constant name")
}

func TestDebug_ReservedStruct_Negative(t *testing.T) {
	expectErr(t, `struct debug { x: int }
fn main() -> int { return 0 }`, "reserved builtin or constant name")
}

func TestDebug_ReservedConst_Negative(t *testing.T) {
	expectErr(t, `const debug: int = 1
fn main() -> int { return debug }`, "reserved builtin or constant name")
}

// TestDebug_TypeVar_Negative pins that debug of a bare generic type parameter is
// a compile error (not a compiler panic): the type-directed renderer cannot be
// selected without a concrete type, consistent with string() rejecting a typevar.
func TestDebug_TypeVar_Negative(t *testing.T) {
	expectErr(t, "fn dbg[T](x: T) -> string { return debug(x) }\n"+wrapMain("return 0"), "generic type")
}

// TestReservedNames_BoundWordsStayDefinable pins AC3b: the context-sensitive
// bound words "comparable" and "numeric" are NOT reserved identifiers, so a
// user may define a function or variable with those names.
func TestReservedNames_BoundWordsStayDefinable(t *testing.T) {
	expectOK(t, `fn comparable(x: int) -> int { return x }
fn main() -> int { return comparable(1) }`)
	expectOK(t, wrapMain(`let numeric: int = 10
print("${numeric}")`))
}

// TestUnreservedKeywords pins that `fallthrough` and `package` are no longer
// reserved keywords: neither will be implemented, and both are once again
// ordinary identifiers usable as function and variable names.
func TestUnreservedKeywords(t *testing.T) {
	expectOK(t, `fn fallthrough(x: int) -> int { return x }
fn main() -> int { return fallthrough(1) }`)
	expectOK(t, wrapMain(`let fallthrough: int = 7
print("${fallthrough}")`))
	expectOK(t, `fn package(x: int) -> int { return x }
fn main() -> int { return package(1) }`)
	expectOK(t, wrapMain(`let package: int = 9
print("${package}")`))
}

// TestDebug_TypeVar_Comparable_Negative is the exact case that previously crashed
// codegen with a nil-deref (comparable-bounded typevar reaches the struct renderer).
func TestDebug_TypeVar_Comparable_Negative(t *testing.T) {
	expectErr(t, "fn dbg[T: comparable](x: T) -> string { return debug(x) }\n"+wrapMain("return 0"), "generic type")
}

// TestBuiltinArity_RangeFormPreserved: a builtin WITH an optional parameter
// (print: msg required, to optional) keeps the "N to M arguments" range form,
// while a fixed-arity builtin uses the singular/plural "N argument(s)" form
// (see TestExtMath_WrongArity for the fixed-arity cases).
func TestBuiltinArity_RangeFormPreserved(t *testing.T) {
	expectErr(t, wrapMain(`print()`), "print expects 1 to 2 arguments, got 0")
	expectErr(t, wrapMain(`print("a", 1, 2)`), "print expects 1 to 2 arguments, got 3")
}

func TestCheckBinaryLogicalNonBoolOperand(t *testing.T) {
	// checkBinary &&/|| arm: a non-bool operand => "requires bool operands".
	// Param operand keeps the expr non-constant so it does not const-fold.
	expectErr(t, `fn f(a: bool) -> int { let x: bool = a && 1; return 0 } fn main() -> int { return 0 }`, "requires bool operands")
	expectErr(t, `fn f(a: int) -> int { let x: bool = a || true; return 0 } fn main() -> int { return 0 }`, "requires bool operands")
}
