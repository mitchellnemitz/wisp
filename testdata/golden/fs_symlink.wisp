import "fs"
fn main() -> int {
  fs.write_file("f", "hello")
  fs.symlink("f", "link")
  print(to_string(fs.is_symlink("link")))
  return 0
}
