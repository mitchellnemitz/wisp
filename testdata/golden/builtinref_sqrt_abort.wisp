import "math"
fn main() -> int {
  let r: fn(float)->float = math.sqrt
  print("${r(-1.0)}")
  return 0
}
