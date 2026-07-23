import "array"

enum Ratio: float { Half = 0.5, Full = 1.0 }

fn main() -> int {
  let xs: Ratio[] = [Ratio.Half, Ratio.Full]
  print("${array.contains(xs, Ratio.Half)}")
  let i: Optional[int] = array.index_of(xs, Ratio.Full)
  print("${unwrap_or(i, -1)}")
  print("${length(array.unique(xs))}")
  assert_eq(Ratio.Half, Ratio.Half)
  assert_ne(Ratio.Half, Ratio.Full)
  assert_contains(xs, Ratio.Full)
  print("ok")
  return 0
}
