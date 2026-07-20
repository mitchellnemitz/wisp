import "fs"
fn main() -> int {
  fs.chmod("missing_file", "644")
  print("unreached")
  return 0
}
