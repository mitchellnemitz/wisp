import "fs"
fn main() -> int {
  fs.write_file("f", "longcontent")
  fs.write_file("f", "x")
  print("[${fs.read_file("f")}]")
  return 0
}
