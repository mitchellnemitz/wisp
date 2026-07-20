import "fs"
fn main() -> int {
  fs.write_file("f", "hello")
  fs.chmod("f", "644")
  print(to_string(fs.is_file("f")))
  return 0
}
