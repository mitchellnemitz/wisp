import "array"
import "fs"
import "string"
fn main() -> int {
  fs.make_dir("d")
  fs.write_file("d/a.txt", "")
  fs.write_file("d/b.txt", "")
  fs.write_file("d/c.log", "")
  let txts: string[] = fs.glob("d/*.txt")
  print(string.join(array.sort(txts), ","))
  print(to_string(length(txts)))
  let none: string[] = fs.glob("d/*.none")
  print(to_string(length(none)))
  return 0
}
