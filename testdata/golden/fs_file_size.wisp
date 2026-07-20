import "fs"
fn main() -> int {
  fs.write_file("f", "hello")
  let n: int = fs.file_size("f")
  print(to_string(n))
  return 0
}
