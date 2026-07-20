import "fs"
fn main() -> int {
  fs.write_file("link", "x")
  fs.symlink_force("new", "link")
  print(unwrap_or(fs.read_link("link"), "none"))
  return 0
}
