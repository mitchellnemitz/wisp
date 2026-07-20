fn eq[T: comparable](a: T, b: T) -> bool { return a == b }
fn main() -> int {
  let a: Optional[int] = Some(1)
  let b: Optional[int] = Some(2)
  let r: bool = eq(a, b)
  return 0
}
