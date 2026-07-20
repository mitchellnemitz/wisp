import "array"
import "math"
fn main() -> int {
  let a: float[] = array.map([-1.0, 4.0], math.sqrt)
  print("${a[0]}")
  return 0
}
