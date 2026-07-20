package codegen

import (
	"strings"
	"testing"
)

// TestTupleLiteralShape: compiled shell uses struct-handle layout (__wisp_s_)
// and stores no runtime length field (tuples have static arity).
func TestTupleLiteralShape(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let t: (int,string) = (1, "a")`)))
	if !strings.Contains(out, "__wisp_s_") {
		t.Errorf("tuple: expected struct-handle layout (__wisp_s_), got:\n%s", out)
	}
	// Assert no array-length field assignment is emitted for a tuple handle.
	if strings.Contains(out, "_len=") {
		t.Errorf("tuple: expected NO _len field assignment, got:\n%s", out)
	}
}

// TestTupleEvalOrder: elements evaluate in source order (AC 11).
func TestTupleEvalOrder(t *testing.T) {
	src := "fn a() -> int { print(\"a\")\nreturn 1 }\n" +
		"fn b() -> int { print(\"b\")\nreturn 2 }\n" +
		"fn main() -> int {\nlet t: (int,int) = (a(), b())\nreturn 0\n}\n"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "a\nb\n" {
		t.Errorf("stdout = %q, want %q", stdout, "a\nb\n")
	}
}

// TestTupleIndexBehavioral: element access returns the correct value at runtime.
func TestTupleIndexBehavioral(t *testing.T) {
	stdout, _, code := run(t, compile(t, wrapMainCG(`let t: (int,string) = (7, "hello")`+"\n"+`print(to_string(t[0]))`+"\n"+`print(t[1])`)))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "7\nhello\n" {
		t.Errorf("stdout = %q, want %q", stdout, "7\nhello\n")
	}
}

// TestTupleLeadingZeroIndex: t[01] reads the same slot as t[1] (AC 4 regression).
// The lexer accepts leading zeros, so genTupleIndex must parse the literal to an
// int and emit strconv.Itoa(k) as the field suffix, matching what genTupleLit
// writes -- never lit.Raw verbatim.
func TestTupleLeadingZeroIndex(t *testing.T) {
	stdout, _, code := run(t, compile(t, wrapMainCG(`let t: (int,string) = (7, "hello")`+"\n"+`print(t[01])`)))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello\n")
	}
}

// TestTupleNestedAccess: t[0][1] chains two IndexExprs (AC 7 nested).
func TestTupleNestedAccess(t *testing.T) {
	stdout, _, code := run(t, compile(t, wrapMainCG(`let t: ((int,bool),string) = ((3, true), "x")`+"\n"+`print(to_string(t[0][0]))`+"\n"+`print(to_string(t[0][1]))`+"\n"+`print(t[1])`)))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "3\ntrue\nx\n" {
		t.Errorf("stdout = %q, want %q", stdout, "3\ntrue\nx\n")
	}
}

// TestTupleFuncParamReturn: tuple as function parameter and return type (AC 12).
func TestTupleFuncParamReturn(t *testing.T) {
	src := "fn pair(a: int, b: string) -> (int,string) { return (a, b) }\n" +
		"fn main() -> int {\nlet r: (int,string) = pair(42, \"hi\")\nprint(to_string(r[0]))\nprint(r[1])\nreturn 0\n}\n"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "42\nhi\n" {
		t.Errorf("stdout = %q, want %q", stdout, "42\nhi\n")
	}
}

// array.zip is a removable builtin (bare zip no longer resolves in the
// single-module check), so the three tests below compile through
// compileNS/runNS with the array namespace bound.

// TestZipBehavioral: zip pairs to min length; element access works at runtime.
func TestZipBehavioral(t *testing.T) {
	// AC 13: length = min(3, 2) = 2; elements are (1,"a") and (2,"b")
	body := `let a: int[] = [1, 2, 3]` + "\n" +
		`let b: string[] = ["a", "b"]` + "\n" +
		`let z: (int,string)[] = array.zip(a, b)` + "\n" +
		`print(to_string(z[0][0]))` + "\n" + `print(z[0][1])` + "\n" +
		`print(to_string(z[1][0]))` + "\n" + `print(z[1][1])`
	stdout, _, code := runNS(t, wrapMainCG(body), "array")
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "1\na\n2\nb\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1\na\n2\nb\n")
	}
}

// TestZipLengthIsMin: the result length equals min of the two input lengths.
func TestZipLengthIsMin(t *testing.T) {
	body := `let a: int[] = [1]` + "\n" + `let b: int[] = [2, 3, 4]` + "\n" +
		`let z: (int,int)[] = array.zip(a, b)` + "\n" + `print(to_string(length(z)))`
	stdout, _, code := runNS(t, wrapMainCG(body), "array")
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "1\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1\n")
	}
}

// TestZipEmptyInput: zip of an empty array yields an empty array.
func TestZipEmptyInput(t *testing.T) {
	body := `let a: int[] = []` + "\n" + `let b: string[] = ["x"]` + "\n" +
		`let z: (int,string)[] = array.zip(a, b)` + "\n" + `print(to_string(length(z)))`
	stdout, _, code := runNS(t, wrapMainCG(body), "array")
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "0\n" {
		t.Errorf("stdout = %q, want %q", stdout, "0\n")
	}
}

// TestTupleReachability: a function called ONLY inside a tuple literal is NOT
// tree-shaken (AC 15).
func TestTupleReachability(t *testing.T) {
	src := "fn compute() -> int { print(\"reached\")\nreturn 99 }\n" +
		"fn main() -> int {\nlet t: (int,int) = (compute(), 1)\nreturn 0\n}\n"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "reached\n" {
		t.Errorf("stdout = %q, want %q", stdout, "reached\n")
	}
}
