package types

import (
	"strings"
	"testing"
)

// generic decls used by the consume tests.
const genPreamble = "" +
	"fn identity[T](x: T) -> T {\n  return x\n}\n" +
	"fn empty_list[T]() -> T[] {\n  let xs: T[] = []\n  return xs\n}\n" +
	"fn both[T, U](a: T, b: U) -> bool {\n  return true\n}\n" +
	"fn eq[T: comparable](a: T, b: T) -> bool {\n  return a == b\n}\n" +
	"fn pick[T: comparable]() -> T {\n  let xs: T[] = []\n  return xs[0]\n}\n" +
	"fn add[T: numeric](a: T, b: T) -> T {\n  return a + b\n}\n" +
	"fn plain(x: int) -> int {\n  return x\n}\n"

// posOf returns the 1-based line/col of the first occurrence of sub in src.
func posOf(src, sub string) (line, col int) {
	for i, ln := range strings.Split(src, "\n") {
		if j := strings.Index(ln, sub); j >= 0 {
			return i + 1, j + 1
		}
	}
	return 0, 0
}

func TestCallTypeArgs_ExplicitBindingOK(t *testing.T) {
	expectOK(t, genPreamble+wrapMain("let v: int = identity[int](42)"))
	// return-only parameter: impossible to infer, works explicitly.
	expectOK(t, genPreamble+wrapMain("let xs: int[] = empty_list[int]()"))
}

func TestCallTypeArgs_ArityErrors(t *testing.T) {
	// too many
	expectErr(t, genPreamble+wrapMain("let v: int = identity[int, string](42)"), "type argument")
	// too few
	expectErr(t, genPreamble+wrapMain("let b: bool = both[int](1, \"s\")"), "type argument")
}

func TestCallTypeArgs_NonGeneric(t *testing.T) {
	expectErr(t, genPreamble+wrapMain("let v: int = plain[int](5)"), "is not generic")
}

func TestCallTypeArgs_InferenceConflict(t *testing.T) {
	d := expectErr(t, genPreamble+wrapMain("let v: int = identity[int](\"s\")"), "explicit type argument")
	// blamed at the value argument, not the type argument.
	src := genPreamble + wrapMain("let v: int = identity[int](\"s\")")
	line, col := posOf(src, "\"s\"")
	if d.Pos.Line != line || d.Pos.Col != col {
		t.Errorf("conflict pos = %d:%d, want value-arg %d:%d", d.Pos.Line, d.Pos.Col, line, col)
	}
}

func TestCallTypeArgs_ComparableBoundViolation(t *testing.T) {
	// float does not satisfy comparable; error anchored at the `float` TYPE ARG.
	src := genPreamble + wrapMain("let b: bool = eq[float](1.0, 2.0)")
	d := expectErr(t, src, "does not satisfy comparable")
	line, col := posOf(src, "float")
	if d.Pos.Line != line || d.Pos.Col != col {
		t.Errorf("bound-error pos = %d:%d, want type-arg %d:%d (proves typeArgPos, not origin/callee)", d.Pos.Line, d.Pos.Col, line, col)
	}
}

func TestCallTypeArgs_ReturnOnlyBoundViolation(t *testing.T) {
	// return-only comparable param: no value arg binds T, so origin is empty; the
	// bound error must still anchor at the `float` type arg.
	src := genPreamble + wrapMain("let v: float = pick[float]()")
	d := expectErr(t, src, "does not satisfy comparable")
	line, col := posOf(src, "pick[float]")
	col += len("pick[") // point at the `float` type arg, not the `let v: float` annotation
	if d.Pos.Line != line || d.Pos.Col != col {
		t.Errorf("return-only bound pos = %d:%d, want type-arg %d:%d", d.Pos.Line, d.Pos.Col, line, col)
	}
}

func TestCallTypeArgs_NumericBoundViolation(t *testing.T) {
	// string does not satisfy numeric; exercises the NUMERIC sweep's typeArgPos.
	src := genPreamble + wrapMain("let s: string = add[string](\"a\", \"b\")")
	d := expectErr(t, src, "does not satisfy numeric")
	line, col := posOf(src, "add[string]")
	// the type arg `string` begins just after "add[".
	col += len("add[")
	if d.Pos.Line != line || d.Pos.Col != col {
		t.Errorf("numeric bound pos = %d:%d, want type-arg %d:%d", d.Pos.Line, d.Pos.Col, line, col)
	}
}

func TestCallTypeArgs_AliasArg(t *testing.T) {
	src := "type MyInt = int\n" + genPreamble + wrapMain("let v: int = identity[MyInt](5)")
	expectOK(t, src)
}

func TestCallTypeArgs_UnknownTypeArg(t *testing.T) {
	expectErr(t, genPreamble+wrapMain("let v: int = identity[Nope](5)"), "Nope")
}

// countErr returns how many errors contain sub.
func countErr(info *Info, sub string) int {
	n := 0
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, sub) {
			n++
		}
	}
	return n
}

// TestCallTypeArgs_ReturnOnlyUnknownTypeArgNoCascade: an unresolvable type arg on a
// return-only generic reports the unknown-type error only -- no "cannot infer"
// cascade (spec error case 10).
func TestCallTypeArgs_ReturnOnlyUnknownTypeArgNoCascade(t *testing.T) {
	info := check(t, genPreamble+wrapMain("let xs: int[] = empty_list[Nope]()"))
	if countErr(info, "Nope") == 0 {
		t.Fatalf("want an unknown-type error, got:\n%s", diagList(info.Errors))
	}
	if n := countErr(info, "cannot infer"); n != 0 {
		t.Errorf("want no cannot-infer cascade, got %d:\n%s", n, diagList(info.Errors))
	}
}

// TestCallTypeArgs_ReturnOnlyArityNoCascade: a type-arg arity mismatch on a generic
// with a return-only param reports the arity error only -- no cannot-infer cascade.
func TestCallTypeArgs_ReturnOnlyArityNoCascade(t *testing.T) {
	// both[T, U] is 2-param; empty_pair below is return-only in both params.
	src := "fn empty_pair[T, U]() -> T[] {\n  let xs: T[] = []\n  return xs\n}\n" +
		wrapMain("let xs: int[] = empty_pair[int]()")
	info := check(t, src)
	if countErr(info, "type argument") == 0 {
		t.Fatalf("want an arity error, got:\n%s", diagList(info.Errors))
	}
	if n := countErr(info, "cannot infer"); n != 0 {
		t.Errorf("want no cannot-infer cascade, got %d:\n%s", n, diagList(info.Errors))
	}
}
