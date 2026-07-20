import "fs"
fn main() -> int {
  fs.make_dir("d")
  fs.write_file("d/f", "x")
  fs.remove_dir("d")
  print("unreached")
  return 0
}
