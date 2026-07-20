import "fs"
fn main() -> int {
  fs.remove_dir("gone")
  print("unreached")
  return 0
}
