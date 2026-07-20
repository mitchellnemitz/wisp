import "fs"
fn main() -> int {
  fs.write_file("x", "")
  fs.make_dir("x/sub")
  print("unreached")
  return 0
}
