import "fs"
fn main() -> int {
  let p: Optional[string] = fs.which("sh")
  print(to_string(is_some(p)))
  print(to_string(length(unwrap(p)) > 0))
  return 0
}
