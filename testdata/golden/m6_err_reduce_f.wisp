import "array"
fn bad(x: int) -> int { return x }
fn main() -> int {
  let xs: int[] = [1, 2]
  let r: int = array.reduce(xs, 0, bad)
  return 0
}
