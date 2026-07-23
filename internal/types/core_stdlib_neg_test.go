package types

import "testing"

// Negative type/overload/domain coverage for removable string/math/array/process/
// env builtins, migrated to namespaced (module) form from stdlib_test.go. The
// bare spellings are gone under PR C; positive member-result coverage lives in the
// core_strings/core_math/core_arrays/core_process/core_env suites, and these are
// the type-mismatch/overload-dispatch assertions those suites do not cover.
// Uses checkNsProg/wantNsErr from core_collections_neg_test.go.

// --- string.index_of overload dispatch ---

// TestCoreStringsPos_IndexOfArrayFloat: float is an admitted comparable
// element type (uniform scalar comparability), so array index_of over a
// float[] type-checks clean.
func TestCoreStringsPos_IndexOfArrayFloat(t *testing.T) {
	wantNsOK(t, "string", `fn main() -> int { let xs: float[] = [1.0, 2.0]; let i: Optional[int] = string.index_of(xs, 1.0); return 0 }`)
}

func TestCoreStringsNeg_IndexOfArrayArgTypeMismatch(t *testing.T) {
	wantNsErr(t, "string", `fn main() -> int { let xs: int[] = [1, 2]; let i: Optional[int] = string.index_of(xs, "x"); return 0 }`, "the array element type")
}

func TestCoreStringsNeg_IndexOfArg0NotStringOrArray(t *testing.T) {
	wantNsErr(t, "string", `fn main() -> int { let i: Optional[int] = string.index_of(42, "x"); return 0 }`, "must be a string or an array")
}

func TestCoreStringsNeg_IndexOfResultOptional(t *testing.T) {
	// index_of yields Optional[int]; binding to a bare int is a mismatch.
	wantNsErr(t, "string", `fn main() -> int { let i: int = string.index_of("abc", "b"); return 0 }`, "want int")
}

// --- string.contains overload dispatch ---

func TestCoreStringsNeg_ContainsStringBadArg2(t *testing.T) {
	wantNsErr(t, "string", `fn main() -> int { let b: bool = string.contains("hello", 1); return 0 }`, "contains")
}

// TestCoreStringsPos_ContainsArrayFloat: float is an admitted comparable
// element type (uniform scalar comparability), so array contains over a
// float[] type-checks clean.
func TestCoreStringsPos_ContainsArrayFloat(t *testing.T) {
	wantNsOK(t, "string", `fn main() -> int { let xs: float[] = [1.0, 2.0]; let b: bool = string.contains(xs, 1.0); return 0 }`)
}

func TestCoreStringsNeg_ContainsArrayStruct(t *testing.T) {
	wantNsErr(t, "string", `struct P { x: int }
fn main() -> int { let xs: P[] = [P { x: 1 }]; let b: bool = string.contains(xs, P { x: 1 }); return 0 }`, "comparable element types int/bool/string")
}

func TestCoreStringsNeg_ContainsArrayOfArray(t *testing.T) {
	wantNsErr(t, "string", `fn main() -> int { let xs: int[][] = [[1], [2]]; let b: bool = string.contains(xs, [1]); return 0 }`, "comparable element types int/bool/string")
}

// --- math.min / math.max / math.abs ---

func TestCoreMathNeg_MinMixIntFloat(t *testing.T) {
	wantNsErr(t, "math", `fn main() -> int { let a: float = math.min(3, 7.0); return 0 }`, "same numeric type")
}

func TestCoreMathNeg_MaxMixFloatInt(t *testing.T) {
	wantNsErr(t, "math", `fn main() -> int { let a: float = math.max(3.0, 7); return 0 }`, "same numeric type")
}

func TestCoreMathNeg_MinNonNumeric(t *testing.T) {
	wantNsErr(t, "math", `fn main() -> int { let a: string = math.min("a", "b"); return 0 }`, "must be int or float")
}

func TestCoreMathNeg_AbsString(t *testing.T) {
	wantNsErr(t, "math", `fn main() -> int { let a: int = math.abs("x"); return 0 }`, "must be int or float")
}

// --- array.reverse / array.reduce ---

func TestCoreArraysNeg_ReverseNonArray(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int { let ys: int[] = array.reverse(1); return 0 }`, "must be an array")
}

func TestCoreArraysNeg_ReduceWrongArity(t *testing.T) {
	wantNsErr(t, "array", `fn bad(x: int) -> int { return x }
fn main() -> int { let xs: int[] = [1, 2]; let r: int = array.reduce(xs, 0, bad); return 0 }`, "two arguments")
}

func TestCoreArraysNeg_ReduceWrongAccType(t *testing.T) {
	wantNsErr(t, "array", `fn bad(acc: string, x: int) -> string { return acc }
fn main() -> int { let xs: int[] = [1, 2]; let r: int = array.reduce(xs, 0, bad); return 0 }`, "accumulator")
}

func TestCoreArraysNeg_ReduceWrongElemType(t *testing.T) {
	wantNsErr(t, "array", `fn bad(acc: int, x: string) -> int { return acc }
fn main() -> int { let xs: int[] = [1, 2]; let r: int = array.reduce(xs, 0, bad); return 0 }`, "element")
}

func TestCoreArraysNeg_ReduceWrongReturnType(t *testing.T) {
	wantNsErr(t, "array", `fn bad(acc: int, x: int) -> string { return "x" }
fn main() -> int { let xs: int[] = [1, 2]; let r: int = array.reduce(xs, 0, bad); return 0 }`, "accumulator type")
}

func TestCoreArraysNeg_ReduceNotFuncref(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int { let xs: int[] = [1, 2]; let r: int = array.reduce(xs, 0, 5); return 0 }`, "function reference")
}

// --- process.exec_command / process.run_input(_full) ---

func TestCoreProcessNeg_ExecCommandArity(t *testing.T) {
	wantNsErr(t, "process", `fn main() -> int { process.exec_command(); return 0 }`, "1 argument")
	wantNsErr(t, "process", `fn main() -> int { process.exec_command(["a"], ["b"]); return 0 }`, "1 argument")
}

func TestCoreProcessNeg_ExecCommandType(t *testing.T) {
	wantNsErr(t, "process", `fn main() -> int { process.exec_command("x"); return 0 }`, "must be string[]")
	wantNsErr(t, "process", `fn main() -> int { process.exec_command([1]); return 0 }`, "must be string[]")
}

func TestCoreProcessNeg_ExecCommandVoidInExpr(t *testing.T) {
	wantNsErr(t, "process", `fn main() -> int { let x: int = process.exec_command(["a"]); return 0 }`, "void")
}

func TestCoreProcessNeg_RunInputArity(t *testing.T) {
	wantNsErr(t, "process", `fn main() -> int { let s: string = process.run_input(["a"]); return 0 }`, "run_input")
	wantNsErr(t, "process", `fn main() -> int { let r: RunResult = process.run_input_full(["a"]); return 0 }`, "run_input_full")
}

func TestCoreProcessNeg_RunInputType(t *testing.T) {
	wantNsErr(t, "process", `fn main() -> int { let s: string = process.run_input("x", "s"); return 0 }`, "must be string[]")
	wantNsErr(t, "process", `fn main() -> int { let r: RunResult = process.run_input_full(["a"], 2); return 0 }`, "must be string")
}

// --- env.set / env.unset ---

func TestCoreEnvNeg_SetArity(t *testing.T) {
	wantNsErr(t, "env", `fn main() -> int { env.set("a"); return 0 }`, "expects")
	wantNsErr(t, "env", `fn main() -> int { env.unset(); return 0 }`, "expects")
}

func TestCoreEnvNeg_SetType(t *testing.T) {
	wantNsErr(t, "env", `fn main() -> int { env.set(1, "b"); return 0 }`, "has type")
	wantNsErr(t, "env", `fn main() -> int { env.unset(3); return 0 }`, "has type")
}

func TestCoreEnvNeg_SetVoidInExpr(t *testing.T) {
	wantNsErr(t, "env", `fn main() -> int { let x: string = env.set("a", "b"); return 0 }`, "void")
	wantNsErr(t, "env", `fn main() -> int { let y: string = env.unset("a"); return 0 }`, "void")
}
