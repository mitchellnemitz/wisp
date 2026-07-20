fn fib(n: int) -> int {
  if (n < 2) { return n }
  return fib(n - 1) + fib(n - 2)
}
fn g() -> int { let z: int = 5; let w: int = 6; return z + w }
fn h(x: int) -> int { return (x + 5) + g() }
fn main() -> int {
  print(to_string(fib(15)))
  print(to_string((1 + 2) + g()))
  print(to_string(h(1)))
  if ((1 + 2) < g()) { print("less") } else { print("notless") }
  return 0
}
