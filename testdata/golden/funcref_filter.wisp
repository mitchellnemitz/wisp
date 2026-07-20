import "array"
fn even(x: int) -> bool { return x % 2 == 0 }
fn main() -> int {
  let xs: int[] = [1, 2, 3, 4, 5, 6]
  let ys: int[] = array.filter(xs, even)
  print("len=${length(ys)}")
  for (y in ys) { print(to_string(y)) }
  return 0
}
