import "fs"
fn main() -> int {
  fs.write_file("f", "abc\n\n\n")
  let c: string = fs.read_file("f")
  print("[${c}]")
  return 0
}
