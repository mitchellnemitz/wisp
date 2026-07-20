import "fs"
import "process"
fn main() -> int {
  fs.write_file("data", "viacat")
  print(process.run(["cat", "data"]))
  return 0
}
