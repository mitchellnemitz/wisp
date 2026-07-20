import "fs"
fn main() -> int {
  fs.symlink_force("/no/such/target", "link")
  print(unwrap_or(fs.read_link("link"), "none"))
  return 0
}
