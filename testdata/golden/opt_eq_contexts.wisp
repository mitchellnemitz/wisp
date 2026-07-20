fn main() -> int {
  let a: Optional[int] = Some(1)
  let b: Optional[int] = Some(1)
  let c: Optional[int] = Some(2)
  let r1: bool = a == b
  if (a == b) { print("if-ok") }
  if (a == b && a != c) { print("and-ok") }
  if (r1) { print("let-ok") }
  return 0
}
