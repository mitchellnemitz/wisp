import "fs"
fn main() -> int {
  fs.write_file("f", "a")
  fs.append_file("f", "b")
  fs.append_file("g", "new")
  print(fs.read_file("f"))
  print(fs.read_file("g"))
  return 0
}
