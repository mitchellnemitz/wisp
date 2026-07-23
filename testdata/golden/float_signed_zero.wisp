import "array"

fn main() -> int {
  print("${array.contains([0.0], -0.0)}")
  print("${array.contains([-0.0], 0.0)}")
  let i: Optional[int] = array.index_of([0.0, 1.0], -0.0)
  print("${unwrap_or(i, -1)}")
  let u: float[] = array.unique([0.0, -0.0, 1.0])
  print("${length(u)}")
  assert_eq(-0.0, 0.0)
  print("ok")
  return 0
}
