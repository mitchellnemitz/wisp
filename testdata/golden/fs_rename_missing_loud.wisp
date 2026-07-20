import "fs"
fn main() -> int {
  fs.rename("gone", "b")
  print("unreached")
  return 0
}
