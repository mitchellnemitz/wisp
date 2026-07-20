fn main() -> int {
  assert(true)
  assert_eq(2 + 2, 4)
  assert_ne("a", "b")
  let o: Optional[int] = Some(7)
  assert_some(o)
  let r: Result[int] = Ok(7)
  assert_ok(r)
  let r2: Result[int] = Err(error("boom"))
  assert_err(r2)
  assert_contains("hello world", "world")
  let xs: int[] = [1, 2, 3]
  assert_contains(xs, 2)
  print("all passed")
  return 0
}
