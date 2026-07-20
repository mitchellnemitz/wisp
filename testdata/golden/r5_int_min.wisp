import "math"
fn main() -> int {
  let m: int = math.int_min()
  print(to_string(m < -1000000000))
  return 0
}
