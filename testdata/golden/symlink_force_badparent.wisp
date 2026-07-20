import "fs"
fn main() -> int {
  fs.symlink_force("t", "/no/such/dir/l")
  return 0
}
