import "array"
fn main() -> int {
  let xs: float[] = [1.0, 2.5]
  print("${array.contains(xs, 1.00)}")
  print("${array.contains(xs, 2.5)}")
  print("${array.contains(xs, 3.0)}")
  let neg: float[] = [0.0]
  print("${array.contains(neg, -0.0)}")
  return 0
}
