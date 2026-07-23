fn inner[T: comparable](a: T, b: T) -> bool { return a == b }
fn outer[U: comparable](x: U, y: U) -> bool { return inner(x, y) }

fn main() -> int {
  print("${outer(1.0, 1.00)}")
  return 0
}
