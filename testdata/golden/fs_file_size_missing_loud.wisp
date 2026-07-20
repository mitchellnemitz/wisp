import "fs"
fn main() -> int {
  let n: int = fs.file_size("missing_path_xyz")
  print(to_string(n))
  return 0
}
