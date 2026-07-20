fn less[T: numeric](a: T, b: T) -> bool { return a < b }
fn main() -> int {
  print(to_string(less(1, 2)))
  print(to_string(less(3, 2)))
  print(to_string(less(1.0, 2.0)))
  print(to_string(less(3.0, 2.0)))
  return 0
}
