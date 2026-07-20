import "fs"
fn main() -> int {
  fs.write_file("a", "hi")
  fs.rename("a", "b")
  print(fs.read_file("b"))
  print(to_string(fs.file_exists("a")))
  return 0
}
