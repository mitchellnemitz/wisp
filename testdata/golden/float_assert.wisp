fn main() -> int {
  assert_eq(1.0, 1.00)
  assert_ne(1.0, 2.0)
  assert_eq(-0.0, 0.0)
  let xs: float[] = [1.0, 2.5]
  assert_contains(xs, 1.00)
  print("ok")
  return 0
}
