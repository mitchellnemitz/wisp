import "string"
fn main() -> int {
  print(string.substring("café", 0, length("café")))
  print(string.substring("café", 0, 3))
  print(string.substring("café", 3, 3))
  return 0
}
