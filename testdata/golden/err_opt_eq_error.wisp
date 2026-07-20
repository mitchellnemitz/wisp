fn main() -> int {
  let a: Optional[error] = Some(error("x", 1))
  let b: Optional[error] = Some(error("y", 2))
  let r: bool = a == b
  return 0
}
