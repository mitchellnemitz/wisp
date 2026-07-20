import "fs"
import "process"
import "string"
fn main() -> int {
  process.run_full(["mkdir", "subdir"])
  fs.change_dir("subdir")
  if (string.ends_with(fs.cwd(), "subdir")) {
    print("ok")
  }
  return 0
}
