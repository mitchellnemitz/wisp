import "math"
fn s(x: float) -> float { return math.sqrt(x) }
fn main() -> int {
  let r: fn(float)->float = math.sqrt
  print("${r(16.0)}")
  print("${math.sqrt(16.0)}")
  print("${s(16.0)}")
  let e: fn(float)->float = math.exp
  print("${e(0.0)}")
  print("${math.exp(0.0)}")
  let l: fn(float)->float = math.ln
  print("${l(1.0)}")
  print("${math.ln(1.0)}")
  return 0
}
