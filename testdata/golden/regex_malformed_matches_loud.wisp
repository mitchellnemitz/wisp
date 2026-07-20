import "regex"
fn main() -> int {
  let m: bool = regex.matches("x", "[")
  print("unreached")
  return 0
}
