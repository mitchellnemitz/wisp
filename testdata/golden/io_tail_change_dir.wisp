import "fs"
fn main() -> int {
  fs.change_dir(".")
  print("ok")
  try {
    fs.change_dir("/nonexistent_wisp_test_path_xyzzy_abcdef")
  } catch (e) {
    print("caught")
  }
  return 0
}
