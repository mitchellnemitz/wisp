fn main() -> int {
  let a: Optional[float] = Some(1.0)
  let b: Optional[float] = Some(1.00)
  print("${a == b}")
  let z1: Optional[float] = Some(-0.0)
  let z2: Optional[float] = Some(0.0)
  print("${z1 == z2}")
  let n1: Optional[Optional[float]] = Some(Some(1.0))
  let n2: Optional[Optional[float]] = Some(Some(1.00))
  print("${n1 == n2}")
  assert_eq(a, b)
  assert_ne(a, Some(2.0))
  assert_eq(n1, n2)
  print("ok")
  return 0
}
