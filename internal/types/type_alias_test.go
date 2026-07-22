package types

import (
	"strings"
	"testing"
)

// errCount returns the number of checker errors for src (parse errors fail).
func errCount(t *testing.T, src string) int {
	t.Helper()
	return len(check(t, src).Errors)
}

// --- Transparency in value positions ---

func TestAliasScalarInterchangeable(t *testing.T) {
	expectOK(t, "type Miles = int\n"+wrapMain(
		"let m: Miles = 5\nlet n: int = m\nlet s: Miles = m + n\nreturn s"))
}

func TestAliasFuncrefInterchangeable(t *testing.T) {
	src := "type BinOp = fn(int, int) -> int\n" +
		"fn add(a: int, b: int) -> int { return a + b }\n" +
		"fn apply(f: BinOp, x: int, y: int) -> int { return f(x, y) }\n" +
		wrapMain("let f: BinOp = add\nreturn apply(f, 2, 3)")
	expectOK(t, src)
}

func TestAliasToStructAnnotationAndFieldAccess(t *testing.T) {
	src := "struct Point { x: int, y: int }\n" +
		"type P = Point\n" +
		wrapMain("let p: P = Point { x: 1, y: 2 }\nlet q: Point = p\nreturn p.x + q.y")
	expectOK(t, src)
}

func TestAliasToEnumInterchangeable(t *testing.T) {
	src := "enum Color: int { Red, Green, Blue }\n" +
		"type C = Color\n" +
		wrapMain("let c: C = Color.Red\nlet d: Color = c\nreturn 0")
	expectOK(t, src)
}

// --- Nesting: alias resolves at any depth ---

func TestAliasNesting(t *testing.T) {
	cases := []string{
		"let a: Miles[] = []\n",
		"let d: {string: Miles} = {}\n",
		"let o: Optional[Miles] = None\n",
	}
	for _, body := range cases {
		expectOK(t, "type Miles = int\n"+wrapMain(body+"return 0"))
	}
}

func TestAliasForwardReference(t *testing.T) {
	// A declared before B resolves (order-independent).
	expectOK(t, "type A = B[]\ntype B = int\n"+wrapMain("let x: A = []\nreturn 0"))
}

// --- typeParam non-poisoning ---

func TestAliasTypeParamNotPoisoned(t *testing.T) {
	// `type Wrap = T` names a bare type parameter at top level; T is not in scope,
	// so it must be an unknown type -- NOT silently the type variable of Box.
	src := "struct Box[T] { m: Wrap }\ntype Wrap = T\n" + wrapMain("return 0")
	expectErr(t, src, "unknown type")
}

// --- Cycles: clean single error, no hang ---

func TestAliasCycles(t *testing.T) {
	main := "\nfn main() -> int { return 0 }"
	structBox := "struct Box[T] { v: T }\n"
	cases := []string{
		"type A = B\ntype B = A" + main,
		"type A = A" + main,
		"type A = (A, int)" + main,
		"type A = A[]" + main,
		"type A = fn(A) -> int" + main,
		structBox + "type A = Box[A]" + main,
	}
	for _, src := range cases {
		info := check(t, src)
		if len(info.Errors) != 1 {
			t.Fatalf("expected exactly one error for cycle, got %d:\n%s\nsrc:\n%s",
				len(info.Errors), diagList(info.Errors), src)
		}
		if !containsMsg(info.Errors, "cycle") {
			t.Errorf("expected a cycle error, got:\n%s\nsrc:\n%s", diagList(info.Errors), src)
		}
	}
}

func containsMsg(ds []Diagnostic, sub string) bool {
	for _, d := range ds {
		if strings.Contains(d.Msg, sub) {
			return true
		}
	}
	return false
}

// --- Reserved-name and collision rejection ---

func TestAliasNameRejections(t *testing.T) {
	main := "\nfn main() -> int { return 0 }"
	cases := []struct{ src, want string }{
		{"type __x = int" + main, "reserved"},
		{"type Result = int" + main, "reserved builtin"},
		{"type _ = int" + main, "cannot be blank"},
		{"struct S { x: int }\ntype S = int" + main, "already declared as a struct"},
		{"enum E: int { A }\ntype E = int" + main, "already declared as an enum"},
		{"type D = int\ntype D = string" + main, "declared more than once"},
	}
	for _, c := range cases {
		expectErr(t, c.src, c.want)
	}
	// A removable builtin name (read_file -> fs.read_file) is freed for reuse: it
	// is no longer a reserved builtin, so a type alias may take that name.
	expectOK(t, "type read_file = int"+main)
}

// --- Unknown RHS, located at the RHS position ---

func TestAliasUnknownRHS(t *testing.T) {
	d := expectErr(t, "type A = Nope[]\n"+wrapMain("return 0"), "unknown type")
	// RHS error is located at the RHS token (line 1), not the alias name.
	if d.Pos.Line != 1 {
		t.Errorf("unknown-RHS error at line %d, want 1 (the RHS)", d.Pos.Line)
	}
}

// --- Generic args on a non-generic alias ---

func TestAliasNotGeneric(t *testing.T) {
	expectErr(t, "type Miles = int\n"+wrapMain("let x: Miles[int] = 0\nreturn 0"),
		"is not generic")
}

// --- Handle-type RHS ---

func TestAliasHandleTypeRHS(t *testing.T) {
	expectOK(t, "type E = error\n"+wrapMain("return 0"))
}

// --- Name-as-syntax limitation (pinned) ---

func TestAliasNotAStructConstructor(t *testing.T) {
	src := "struct Point { x: int, y: int }\n" +
		"type P = Point\n" +
		wrapMain("let p: Point = P { x: 1, y: 2 }\nreturn 0")
	expectErr(t, src, "unknown struct type")
}

func TestAliasNotAnEnumBase(t *testing.T) {
	src := "enum Color: int { Red, Green }\n" +
		"type C = Color\n" +
		wrapMain("let v: Color = C.Red\nreturn 0")
	// The alias name is not an enum name, so the base ident is undeclared.
	expectErr(t, src, "undeclared name")
}

// --- did-you-mean suggests an alias name ---

func TestAliasDidYouMean(t *testing.T) {
	d := expectErr(t, "type Miles = int\n"+wrapMain("let m: Mile = 5\nreturn 0"), "unknown type")
	if !strings.Contains(d.Msg, "Miles") {
		t.Errorf("expected did-you-mean to suggest Miles, got: %q", d.Msg)
	}
}
