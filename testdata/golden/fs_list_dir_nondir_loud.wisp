import "fs"
fn main() -> int {
  fs.write_file("f", "x")
  let xs: string[] = fs.list_dir("f")
  print("unreached")
  return 0
}
