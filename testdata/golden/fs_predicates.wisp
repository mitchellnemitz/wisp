import "fs"
import "process"
fn main() -> int {
  fs.make_dir("d")
  print(to_string(fs.file_exists("d")))
  print(to_string(fs.is_dir("d")))
  print(to_string(fs.file_exists("nope")))
  print(to_string(fs.is_dir("nope")))
  fs.write_file("d/f", "x")
  print(to_string(fs.file_exists("d/f")))
  print(to_string(fs.is_dir("d/f")))
  let setup: string = process.run(["ln", "-s", "d", "link"])
  print(to_string(fs.file_exists("link")))
  print(to_string(fs.is_dir("link")))
  return 0
}
