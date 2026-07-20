import "fs"
fn main() -> int {
  fs.symlink_force("$(touch PWNED); `id`; *", "link")
  print(unwrap_or(fs.read_link("link"), "none"))
  return 0
}
