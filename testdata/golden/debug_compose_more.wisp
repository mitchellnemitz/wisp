fn show(s: string) -> int {
  print(s)
  return 0
}
fn main() -> int {
  let o: Optional[int] = Some(1)
  print("d=${debug(o)}")
  show(debug(o))
  return 0
}
