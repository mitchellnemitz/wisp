fn f(a: int, b: int = 2, c: int = 3) -> int {
  return a + b + c
}
fn main() -> int {
  return f(1, , 9)
}
