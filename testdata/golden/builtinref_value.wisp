import "fs"
import "string"
fn main() -> int {
  let f: fn(string)->string = string.trim
  print(f("  hi  "))
  let u: fn(string)->string = string.upper
  print(u("abc"))
  let lo: fn(string)->string = string.lower
  print(lo("ABC"))
  let b: fn(string)->string = fs.base_name
  print(b("/x/y/z.txt"))
  let d: fn(string)->string = fs.dir_name
  print(d("/x/y/z.txt"))
  return 0
}
