import "fs"
fn main() -> int {
  let f: string = fs.temp_file()
  print(to_string(fs.is_file(f)))
  let d: string = fs.temp_dir()
  print(to_string(fs.is_dir(d)))
  fs.remove_file(f)
  fs.remove_dir(d)
  return 0
}
