import "regex"
fn main() -> int {
  let p: Optional[string] = regex.find("0xyz", "0...")
  print(unwrap(p))
  let q: Optional[string] = regex.find("1abc", "1...")
  print(unwrap(q))
  return 0
}
