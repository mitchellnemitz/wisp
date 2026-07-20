import "fs"
fn main() -> int {
  fs.make_dir("d")
  fs.remove_file("d")
  print("unreached")
  return 0
}
