import "math"
fn main() -> int {
  try {
    print("${math.abs(math.int_min())}")
  } catch (e) {
    print("caught abs")
  }
  try {
    print("${math.int_min() / -1}")
  } catch (e) {
    print("caught div")
  }
  try {
    print("${math.gcd(math.int_min(), 6)}")
  } catch (e) {
    print("caught gcd")
  }
  print("${math.gcd(-12, 18)}")
  print("${math.lcm(4, 6)}")
  return 0
}
