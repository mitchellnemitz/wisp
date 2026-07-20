import "string"
fn main() -> int {
  print(string.char_at("café", 0))
  print(string.char_at("café", 3) + string.char_at("café", 4))
  return 0
}
