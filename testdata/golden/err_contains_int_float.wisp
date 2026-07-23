import "array"
fn main() -> int {
  let xs: int[] = [1, 2]
  print("${array.contains(xs, 1.0)}")
  return 0
}
