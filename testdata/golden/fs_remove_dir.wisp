import "fs"
fn main() -> int {
  fs.make_dir("d")
  fs.remove_dir("d")
  print(to_string(fs.is_dir("d")))
  return 0
}
