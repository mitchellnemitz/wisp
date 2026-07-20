import "math"
fn main() -> int {
  try {
    let r: float = math.pow(-2.0, 0.5)
    print(to_string(r))
  } catch (e) {
    print("caught")
  }
  return 0
}
