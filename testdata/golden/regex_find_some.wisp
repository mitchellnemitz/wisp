import "regex"
fn main() -> int {
  let p: Optional[string] = regex.find("a1b22c", "[0-9]+")
  print(to_string(is_some(p)))
  print(unwrap(p))
  return 0
}
