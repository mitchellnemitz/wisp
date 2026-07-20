import "array"
fn dbl(x: int) -> int { return x * 2 }
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  let ys: int[] = array.map(xs, dbl)
  for (y in ys) { print(to_string(y)) }
  return 0
}
