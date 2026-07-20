import "fs"
import "string"
fn main() -> int {
  fs.write_file("marker", "x")
  print(to_string(fs.file_exists(fs.cwd() + "/marker")))
  print(to_string(string.starts_with(fs.cwd(), "/")))
  return 0
}
