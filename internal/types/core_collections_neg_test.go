package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// Negative type/domain coverage for removable collection + math builtins,
// migrated to namespaced (module) form. The bare spellings that previously
// exercised these paths (in collections_test.go) are gone under PR C, so these
// assertions moved here where CheckLinked can resolve the module member. This
// file is the home for the type-mismatch messages that the core_* member-result
// suites do not otherwise assert.

// checkNsProg links a root program against a single synthetic core module bound
// to namespace ns at id 1, and returns the resulting Info.
func checkNsProg(t *testing.T, ns, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{ns: 1})
	m := coreMod(1, ns)
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, m}})
}

func wantNsErr(t *testing.T, ns, rootSrc, substr string) {
	t.Helper()
	info := checkNsProg(t, ns, rootSrc)
	for _, e := range info.Errors {
		if strings.Contains(e.Msg, substr) {
			return
		}
	}
	t.Fatalf("expected an error containing %q for %q, got: %v", substr, rootSrc, errMsgs(info))
}

// wantNsOK links a root program against a synthetic core module and asserts the
// program type-checks with no errors, returning the Info for further inspection.
func wantNsOK(t *testing.T, ns, rootSrc string) *Info {
	t.Helper()
	info := checkNsProg(t, ns, rootSrc)
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors for %q, got: %v", rootSrc, errMsgs(info))
	}
	return info
}

// firstNsErr is like wantNsErr but returns the matching Diagnostic so callers can
// assert on its source position.
func firstNsErr(t *testing.T, ns, rootSrc, substr string) Diagnostic {
	t.Helper()
	info := checkNsProg(t, ns, rootSrc)
	for _, e := range info.Errors {
		if strings.Contains(e.Msg, substr) {
			return e
		}
	}
	t.Fatalf("expected an error containing %q for %q, got: %v", substr, rootSrc, errMsgs(info))
	return Diagnostic{}
}

func TestCoreArraysNeg_SortNonComparable(t *testing.T) {
	// bool[] now sorts (bool joined the comparable scalar set, SC-004); only a
	// non-scalar element type (int[][] whose element is an array handle) is
	// rejected, with the widened message (SC-010c).
	expectOKNS(t, `fn main() -> int { let xs: bool[] = [true, false]; let s: bool[] = array.sort(xs); return 0 }`, "array")
	wantNsErr(t, "array", `fn main() -> int { let xs: int[][] = [[1]]; let s: int[][] = array.sort(xs); return 0 }`, "ordered scalar type")
}

func TestCoreArraysNeg_ConcatMismatch(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int { let a: int[] = [1]; let b: string[] = ["x"]; let c: int[] = array.concat(a, b); return 0 }`, "same element type")
}

func TestCoreArraysNeg_SumRejectsString(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int { let xs: string[] = ["a"]; let s: string = array.sum(xs); return 0 }`, "int or float")
}

func TestCoreArraysNeg_FindRequiresBoolFunc(t *testing.T) {
	wantNsErr(t, "array", `fn toStr(x: int) -> string { return to_string(x) }
fn main() -> int { let xs: int[] = [1]; let n: Optional[int] = array.find(xs, toStr); return 0 }`, "must return bool")
}

func TestCoreArraysNeg_FindReturnsOptional(t *testing.T) {
	// find yields Optional[elem]; binding to the bare element type is a mismatch.
	wantNsErr(t, "array", `fn isPos(x: int) -> bool { return x > 0 }
fn main() -> int { let xs: int[] = [1]; let n: int = array.find(xs, isPos); return 0 }`, "Optional[int]")
}

func TestCoreArraysNeg_SortByComparatorArity(t *testing.T) {
	wantNsErr(t, "array", `fn bad(a: int) -> bool { return a > 0 }
fn main() -> int { let xs: int[] = [1]; let s: int[] = array.sort_by(xs, bad); return 0 }`, "must be fn(int,int)->bool")
}

func TestCoreDictNeg_GetTypeChecks(t *testing.T) {
	wantNsErr(t, "dict", `fn main() -> int { let d: {string: int} = { "a": 1 }; let g: Optional[int] = dict.get(d, 5); return 0 }`, "the dict key type")
}

func TestCoreDictNeg_MergeMismatch(t *testing.T) {
	wantNsErr(t, "dict", `fn main() -> int { let a: {string: int} = { "a": 1 }; let b: {string: string} = { "b": "x" }; let m: {string: int} = dict.merge(a, b); return 0 }`, "same type")
}

func TestCoreMathNeg_ClampTyping(t *testing.T) {
	// result type follows the operands: an int clamp is not a float.
	wantNsErr(t, "math", `fn main() -> int { let f: float = math.clamp(5, 1, 10); return 0 }`, "has type int, want float")
	// mixed numeric types are rejected (no implicit coercion).
	wantNsErr(t, "math", `fn main() -> int { let x: int = math.clamp(5, 1.0, 10); return 0 }`, "same numeric type")
	// non-numeric argument is rejected.
	wantNsErr(t, "math", `fn main() -> int { let s: string = math.clamp("a", "b", "c"); return 0 }`, "must be int or float")
}

func TestCoreMathNeg_SignTyping(t *testing.T) {
	wantNsErr(t, "math", `fn main() -> int { let s: int = math.sign("x"); return 0 }`, "must be int or float")
}

func TestCoreMathNeg_GcdBothIntMinSingleDiagnostic(t *testing.T) {
	// gcd(INT_MIN, INT_MIN) produces exactly ONE overflow diagnostic, at the first
	// operand (migrated from argdomain_test.go; gcd is now math.gcd).
	info := checkNsProg(t, "math", `fn main() -> int { let g: int = math.gcd(-9223372036854775808, -9223372036854775808); return 0 }`)
	n := 0
	var first Diagnostic
	for _, d := range info.Errors {
		if d.Msg == "math.gcd(): integer overflow" {
			if n == 0 {
				first = d
			}
			n++
		}
	}
	if n != 1 {
		t.Fatalf("gcd(INT_MIN, INT_MIN): want exactly 1 overflow diagnostic, got %d (all: %v)", n, errMsgs(info))
	}
	if first.Pos.Line == 0 || first.Pos.Col == 0 {
		t.Errorf("diagnostic not located (line=%d col=%d)", first.Pos.Line, first.Pos.Col)
	}
}

func TestCoreArraysNeg_PushElemTypeMismatch(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int { let xs: int[] = [1]; array.push(xs, "x"); return 0 }`, "argument 2 of array.push has type string")
}

func TestCoreDictNeg_HasWrongKeyType(t *testing.T) {
	wantNsErr(t, "dict", `fn main() -> int { let m: {string: int} = {}; let b: bool = dict.has(m, 3); return 0 }`, "argument 2 of dict.has")
}

func TestCoreDictNeg_HasNonDict(t *testing.T) {
	wantNsErr(t, "dict", `fn main() -> int { let xs: int[] = [1]; let b: bool = dict.has(xs, 0); return 0 }`, "argument 1 of dict.has must be a dict")
}

func TestCoreDictNeg_KeysNonDict(t *testing.T) {
	wantNsErr(t, "dict", `fn main() -> int { let xs: int[] = [1]; let ks: int[] = dict.keys(xs); return 0 }`, "argument 1 of dict.keys must be a dict")
}

func TestCoreArraysNeg_IsEmptyNonArray(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int { let m: {string: int} = {}; let b: bool = array.is_empty(m); return 0 }`, "argument 1 of array.is_empty must be an array")
}

func TestCoreDictNeg_IsEmptyNonDict(t *testing.T) {
	wantNsErr(t, "dict", `fn main() -> int { let xs: int[] = [1]; let b: bool = dict.is_empty(xs); return 0 }`, "argument 1 of dict.is_empty must be a dict")
}
