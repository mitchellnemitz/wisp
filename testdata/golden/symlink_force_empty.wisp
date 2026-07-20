import "fs"
fn main() -> int {
  fs.symlink_force("t", "")
  return 0
}
