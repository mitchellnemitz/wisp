import "fs"
fn main() -> int {
  print(fs.read_file("does_not_exist"))
  return 0
}
