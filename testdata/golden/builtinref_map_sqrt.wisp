import "array"
import "math"
fn s(x: float) -> float { return math.sqrt(x) }
fn main() -> int {
  let a: float[] = array.map([4.0, 9.0], math.sqrt)
  print("${a[0]}")
  print("${a[1]}")
  let b: float[] = array.map([4.0, 9.0], s)
  print("${b[0]}")
  print("${b[1]}")
  return 0
}
