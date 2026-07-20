import "fs"
fn main() -> int {
  let xs: string[] = fs.list_dir("gone")
  print("unreached")
  return 0
}
