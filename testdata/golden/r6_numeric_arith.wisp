fn add[T: numeric](a: T, b: T) -> T { return a + b }
fn mul[T: numeric](a: T, b: T) -> T { return a * b }
fn main() -> int {
  print(to_string(add(3, 4)))
  print(to_string(mul(6, 7)))
  print(to_string(add(1.5, 2.5)))
  print(to_string(mul(2.0, 3.0)))
  return 0
}
