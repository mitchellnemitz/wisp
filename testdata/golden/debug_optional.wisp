fn main() -> int {
  let s: Optional[int] = Some(42)
  let n: Optional[int] = None
  print(debug(s))
  print(debug(n))
  return 0
}
