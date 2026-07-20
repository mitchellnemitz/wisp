package types

import "testing"

func TestInterp_Array_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1, 2, 3]
print("${xs}")`), "cannot interpolate an array value (it is opaque); use debug(xs) or join")
}

func TestInterp_Dict_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let d: {string:int} = {"a": 1}
print("${d}")`), "cannot interpolate a dict value (it is opaque); use debug(d)")
}

func TestInterp_Struct_Negative(t *testing.T) {
	expectErr(t, `struct Pt { x: int }
fn main() -> int {
  let p: Pt = Pt { x: 1 }
  print("${p}")
  return 0
}`, "cannot interpolate a struct value (it is opaque); use debug(v)")
}

func TestInterp_Tuple_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let tp: (int, string) = (7, "hello")
print("${tp}")`), "cannot interpolate a tuple value (it is opaque); use debug(t)")
}

func TestInterp_Result_Negative(t *testing.T) {
	expectErr(t, wrapMain(`let r: Result[int] = Ok(1)
print("${r}")`), "cannot interpolate a Result value (it is opaque); use debug(r) or match")
}

func TestInterp_Scalar_StillOK(t *testing.T) {
	expectOK(t, wrapMain(`let i: int = 1
let f: float = 1.5
let b: bool = true
let s: string = "hi"
print("${i} ${f} ${b} ${s}")`))
}
