import "math"
fn main() -> int {
  let r: int = math.random(1)
  print(to_string(r == 0))
  let r2: int = math.random(100)
  print(to_string(r2 >= 0 && r2 < 100))
  return 0
}
