import "fs"
fn main() -> int {
  fs.write_file("f", "hello")
  fs.write_file("existing", "")
  fs.symlink("f", "existing")
  print("unreached")
  return 0
}
