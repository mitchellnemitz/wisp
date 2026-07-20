fn countdown(n: int) -> int {
  if (n <= 0) {
    return 0
  }
  return countdown(n - 1)
}
fn main() -> int {
  print(to_string(countdown(800)))
  return 0
}
