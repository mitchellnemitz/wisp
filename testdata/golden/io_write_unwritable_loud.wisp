import "fs"
fn main() -> int {
  fs.write_file("no_such_dir/f", "x")
  return 0
}
