import "fs"
import "process"
fn main() -> int {
  let setup: string = process.run(["sh", "-c", "printf 'a\\000b' > nulf"])
  print(fs.read_file("nulf"))
  return 0
}
