import "array"
fn main() -> int {
  let xs: float[] = [1.0, 2.5, 1.00]
  let i: Optional[int] = array.index_of(xs, 1.00)
  print("${unwrap_or(i, -1)}")
  let u: float[] = array.unique(xs)
  print("${length(u)}")
  let z: float[] = [0.0]
  print("${unwrap_or(array.index_of(z, -0.0), -1)}")
  return 0
}
