import "array"
fn main() -> int {
  let xs: int[] = [1, 2]
  let i: Optional[int] = array.index_of(xs, 1.0)
  return 0
}
