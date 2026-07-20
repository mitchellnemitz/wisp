package types

import "testing"

// --- M3 PR-B: struct/array/main-args/handle-opacity type checking ---

func TestStruct_DeclareConstructAccess_OK(t *testing.T) {
	expectOK(t, `struct Point { x: int, y: int }
fn main() -> int {
  let p: Point = Point { x: 1, y: 2 }
  let z: int = p.x + p.y
  p.x = 10
  return 0
}`)
}

func TestStruct_AsParamAndReturn_OK(t *testing.T) {
	expectOK(t, `struct Box { v: int }
fn make(n: int) -> Box { return Box { v: n } }
fn extract(b: Box) -> int { return b.v }
fn main() -> int {
  let b: Box = make(3)
  return extract(b)
}`)
}

func TestStruct_MissingField_Negative(t *testing.T) {
	expectErr(t, `struct P { x: int, y: int }
fn main() -> int { let p: P = P { x: 1 }
return 0 }`, "missing field \"y\"")
}

func TestStruct_UnknownField_Negative(t *testing.T) {
	expectErr(t, `struct P { x: int }
fn main() -> int { let p: P = P { x: 1, z: 2 }
return 0 }`, "has no field \"z\"")
}

func TestStruct_FieldTypeMismatch_Negative(t *testing.T) {
	expectErr(t, `struct P { x: int }
fn main() -> int { let p: P = P { x: "s" }
return 0 }`, "field \"x\" has type string, want int")
}

func TestStruct_AccessNonStruct_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 1
let z: int = n.x`), "non-struct")
}

func TestStruct_UnknownType_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let p: Nope = Nope { x: 1 }`), "unknown type")
}

func TestStruct_DuplicateField_Negative(t *testing.T) {
	expectErr(t, `struct P { x: int, x: int }
fn main() -> int { return 0 }`, "declared more than once")
}

func TestStruct_NestedField_OK(t *testing.T) {
	expectOK(t, `struct Inner { v: int }
struct Outer { inner: Inner, tags: string[] }
fn main() -> int {
  let o: Outer = Outer { inner: Inner { v: 1 }, tags: ["a"] }
  let z: int = o.inner.v
  let s: string = o.tags[0]
  return 0
}`)
}

func TestArray_LiteralIndexMutateLengthPush_OK(t *testing.T) {
	// push is now removable (array.push; covered in core_arrays_test.go); this test
	// retains the core-language index/mutate/length coverage.
	expectOK(t, wrapMain(`let xs: int[] = [1, 2, 3]
let a: int = xs[0]
xs[1] = 9
let n: int = length(xs)`))
}

func TestArray_EmptyLiteralNeedsAnnotation_OK(t *testing.T) {
	expectOK(t, wrapMain(`let xs: int[] = []
let n: int = length(xs)`))
}

func TestArray_ElementTypeMismatch_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1, "two"]`), "element 2 has type string")
}

func TestArray_NonIntIndex_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1]
let s: string = "0"
let z: int = xs[s]`), "index must be int")
}

// push element-type mismatch is now removable (array.push); coverage moved to
// core_collections_neg_test.go (TestCoreArraysNeg_PushElemTypeMismatch).

func TestArray_LengthOfStringStillWorks_OK(t *testing.T) {
	expectOK(t, wrapMain(`let n: int = length("hello")`))
}

func TestArray_NestedTypes_OK(t *testing.T) {
	expectOK(t, wrapMain(`let g: int[][] = [[1, 2], [3]]
let row: int[] = g[0]
let v: int = g[0][1]`))
}

func TestForIn_BlockScoped_NotVisibleAfter_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1]
for (x in xs) { print(to_string(x)) }
print(to_string(x))`), "undeclared name \"x\"")
}

func TestForIn_SiblingReuse_OK(t *testing.T) {
	expectOK(t, wrapMain(`let xs: int[] = [1]
let ys: int[] = [2]
for (x in xs) { print(to_string(x)) }
for (x in ys) { print(to_string(x)) }`))
}

func TestForIn_NonArray_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 1
for (x in n) { print(to_string(x)) }`), "for-in requires an array")
}

func TestMainArgs_OK(t *testing.T) {
	info := expectOK(t, `fn main(args: string[]) -> int {
  let n: int = length(args)
  let head: string = args[0]
  for (a in args) { print(a) }
  return 0
}`)
	if !info.MainArgs {
		t.Error("expected MainArgs to be true")
	}
}

func TestMainZeroArg_OK(t *testing.T) {
	info := expectOK(t, `fn main() -> int { return 0 }`)
	if info.MainArgs {
		t.Error("expected MainArgs false for zero-arg main")
	}
}

func TestMainBadParamType_Negative(t *testing.T) {
	expectErr(t, `fn main(x: int) -> int { return x }`, "string[]")
}

// --- handle opacity (spec 4.1 handle soundness) ---

func TestHandle_IntConversion_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1]
let n: int = to_int(xs)`), "argument 1 of to_int")
}

func TestHandle_Arithmetic_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1]
let n: int = xs + 1`), "got [int] and int")
}

func TestHandle_ArrayComparison_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let a: int[] = [1]
let b: int[] = [2]
let c: bool = a == b`), "opaque")
}

func TestHandle_StructComparison_Negative(t *testing.T) {
	expectErr(t, `struct P { x: int }
fn main() -> int {
  let a: P = P { x: 1 }
  let b: P = P { x: 1 }
  let c: bool = a != b
  return 0
}`, "opaque")
}

func TestStruct_ReservedName_Negative(t *testing.T) {
	expectErr(t, `struct length { x: int }
fn main() -> int { return 0 }`, "reserved")
}

// --- M3 PR-C: dict type checking ---

func TestDict_LiteralLookupSet_OK(t *testing.T) {
	// has/keys are now removable (dict module; covered in core_dict_test.go); this
	// retains the core-language dict literal/lookup/index-assign coverage.
	expectOK(t, wrapMain(`let m: {string: int} = { "a": 1, "b": 2 }
let v: int = m["a"]
m["c"] = 3`))
}

func TestDict_EmptyLiteralNeedsAnnotation_OK(t *testing.T) {
	expectOK(t, wrapMain(`let m: {int: string} = {}
m[1] = "x"`))
}

func TestDict_EmptyLiteralNoAnnotation_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let v: int = length(keys({}))`), "empty dict literal")
}

func TestDict_IntKeys_OK(t *testing.T) {
	expectOK(t, wrapMain(`let m: {int: string} = { 1: "one", 2: "two" }
let s: string = m[1]
for (k in m) { print(to_string(k)) }`))
}

func TestDict_KeyTypeMismatch_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {string: int} = { "a": 1 }
let v: int = m[5]`), "dict key must be string")
}

func TestDict_ValueTypeMismatch_Negative(t *testing.T) {
	// First-entry value type drives the inferred dict type, so a wrong first value
	// surfaces as the let-binding mismatch; a wrong LATER value surfaces directly.
	expectErr(t, wrapMain(`let m: {string: int} = { "a": 1, "b": "x" }`), "value 2 has type string")
}

func TestDict_DuplicateLiteralKey_String_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {string: int} = { "a": 1, "a": 2 }`), "duplicate key")
}

func TestDict_DuplicateLiteralKey_Int_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {int: int} = { 5: 1, 05: 2 }`), "duplicate key")
}

func TestDict_NonHashableKeyType_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {bool: int} = {}`), "dict key type must be int or string")
}

func TestDict_IndexAssignTypeMismatch_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {string: int} = {}
m["a"] = "nope"`), "cannot assign string")
}

// has/keys are now removable (dict module). Positive keys-result coverage lives
// in core_dict_test.go; the has/keys non-dict and wrong-key-type negatives moved
// to core_collections_neg_test.go (TestCoreDictNeg_*).

func TestDict_ForInBindsKeyType_Negative(t *testing.T) {
	// the loop key is a string; using it where an int is needed is a type error.
	expectErr(t, wrapMain(`let m: {string: int} = { "a": 1 }
for (k in m) { let n: int = k + 1 }`), "got string and int")
}

func TestDict_ForInBlockScoped_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {string: int} = { "a": 1 }
for (k in m) { print(k) }
print(k)`), "undeclared name \"k\"")
}

func TestDict_NestedValueArray_OK(t *testing.T) {
	expectOK(t, wrapMain(`let m: {string: int[]} = { "a": [1, 2], "b": [3] }
let row: int[] = m["a"]
let v: int = m["a"][1]`))
}

func TestDict_HandleOpaque_Comparison_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let a: {string: int} = {}
let b: {string: int} = {}
let c: bool = a == b`), "opaque")
}

func TestDict_HandleOpaque_IntConversion_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let m: {string: int} = {}
let n: int = to_int(m)`), "argument 1 of to_int")
}
