import "array"
fn main() -> int {
  let xs: int[] = [3, 4, 5]
  xs[1] = 40
  array.push(xs, 6)
  let total: int = 0
  for (x in xs) {
    total = total + x
  }
  print("sum=${total}")
  print("len=${length(xs)}")
  print("first=${xs[0]}")
  return 0
}
