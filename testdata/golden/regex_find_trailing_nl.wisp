import "regex"
fn main() -> int {
  let p: Optional[string] = regex.find("x\n", "x.")
  print(to_string(is_some(p)))
  print(unwrap(p))
  return 0
}
