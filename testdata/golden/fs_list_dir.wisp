import "array"
import "fs"
import "process"
import "string"
fn main() -> int {
  fs.make_dir("d")
  fs.write_file("d/a b", "")
  fs.write_file("d/c*d", "")
  fs.write_file("d/.hidden", "")
  let setup: string = process.run(["ln", "-s", "nonexistent_target", "d/brokenlink"])
  print(string.join(array.sort(fs.list_dir("d")), ","))
  return 0
}
