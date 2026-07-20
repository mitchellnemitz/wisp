import "fs"
fn main() -> int {
  fs.symlink("old", "link")
  fs.symlink_force("new", "link")
  print(unwrap_or(fs.read_link("link"), "none"))
  return 0
}
