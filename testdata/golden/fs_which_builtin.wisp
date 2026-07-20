import "fs"
fn main() -> int {
  let p: Optional[string] = fs.which("cd")
  print(to_string(is_some(p)))
  return 0
}
