import "fs"
fn main() -> int {
  fs.make_dir("e")
  print(to_string(length(fs.list_dir("e"))))
  return 0
}
