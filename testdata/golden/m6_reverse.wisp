import "array"
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  let ys: int[] = array.reverse(xs)
  for (y in ys) { print("${y}") }
  print("len=${length(xs)}")
  let e: int[] = []
  print("rev_empty=${length(array.reverse(e))}")
  return 0
}
