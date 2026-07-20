import "fs"
fn main() -> int {
  fs.make_dir("a/b/c")
  print(to_string(fs.is_dir("a/b/c")))
  fs.make_dir("a/b/c")
  print("ok")
  return 0
}
