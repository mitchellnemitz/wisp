import "fs"
fn main() -> int {
  fs.write_file("f", "x")
  fs.remove_file("f")
  print(to_string(fs.file_exists("f")))
  fs.remove_file("gone")
  print("ok")
  return 0
}
