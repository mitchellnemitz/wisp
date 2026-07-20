import "fs"
import "process"
fn main() -> int {
  process.run_full(["sh", "-c", "ln -s . link"])
  fs.change_dir("link")
  print("ok")
  return 0
}
