import "string"
fn main() -> int {
  print(string.repeat("ab", 3))
  print("[${string.repeat("x", 0)}]")
  return 0
}
