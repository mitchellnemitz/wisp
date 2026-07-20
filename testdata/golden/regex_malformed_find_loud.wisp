import "regex"
fn main() -> int {
  let p: Optional[string] = regex.find("x", "[")
  print("unreached")
  return 0
}
