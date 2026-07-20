import "fs"
fn main() -> int {
  fs.make_dir("d")
  let n: int = fs.file_size("d")
  print(to_string(n))
  return 0
}
