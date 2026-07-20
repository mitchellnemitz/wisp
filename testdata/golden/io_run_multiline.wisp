import "fs"
import "process"
fn main() -> int {
  fs.write_file("m", "a\nb\n\n")
  print("[${process.run(["cat", "m"])}]")
  return 0
}
