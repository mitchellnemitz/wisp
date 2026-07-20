fn mark(n: int) -> int {
  print("mark${n}")
  return n
}
fn add(a: int, b: int) -> int {
  return a + b
}
fn main() -> int {
  let r: int = add(mark(1), mark(2))
  print("sum${r}")
  return 0
}
