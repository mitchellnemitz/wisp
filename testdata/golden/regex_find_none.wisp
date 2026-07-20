import "regex"
fn main() -> int {
  let p: Optional[string] = regex.find("abc", "[0-9]+")
  print(to_string(is_none(p)))
  return 0
}
