import "fs"
import "process"
fn main() -> int {
  let setup: string = process.run(["sh", "-c", "printf 'a\\000b' > nulf"])
  try {
    print(fs.read_file("nulf"))
  } catch (e) {
    print("caught nul")
  }
  return 0
}
