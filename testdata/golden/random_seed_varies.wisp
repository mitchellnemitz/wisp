import "math"
fn main() -> int {
  let a: int = math.random(1000000000)
  let b: int = math.random(1000000000)
  let c: int = math.random(1000000000)
  let d: int = math.random(1000000000)
  let e: int = math.random(1000000000)
  let varies: bool = b != a || c != a || d != a || e != a
  print(to_string(varies))
  return 0
}
