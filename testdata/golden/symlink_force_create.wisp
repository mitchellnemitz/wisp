import "fs"
fn main() -> int {
  fs.symlink_force("/some/target", "link")
  print(unwrap_or(fs.read_link("link"), "none"))
  return 0
}
