fn inner[T: comparable](a: T, b: T) -> bool { return a == b }
fn outer[U: comparable](x: U, y: U) -> bool { return inner(x, y) }

fn main() -> int {
  print("${outer("ab", "ab")}")
  print("${outer("ab", "cd")}")
  return 0
}
