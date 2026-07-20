import "math"
fn main() -> int {
  let r: float = math.pow(4.0, 0.5)
  let rounded: int = math.round(r)
  print(to_string(rounded))
  return 0
}
