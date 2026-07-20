import "fs"
fn main() -> int {
  fs.write_file("x", "")
  try {
    fs.make_dir("x/sub")
  } catch (e) {
    print("caught make_dir")
  }
  return 0
}
