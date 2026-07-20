import "array"
import "math"
fn s(x: float) -> float { return math.sqrt(x) }
fn main() -> int {
  let a: float[] = array.map([-1.0, 4.0], s)
  print("${a[0]}")
  return 0
}
