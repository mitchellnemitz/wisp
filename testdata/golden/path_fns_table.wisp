// dir_name/base_name produce the spec P4 table identically on all four shells,
// as pure string functions (no external dirname/basename process). Each line is
// dir_name(input) + "|" + base_name(input).
import "fs"
fn show(p: string) -> void {
  print(fs.dir_name(p) + "|" + fs.base_name(p))
}

fn main() -> int {
  show("/a/b/c")
  show("/a/b/c/")
  show("a/b")
  show("b")
  show("/")
  show("//")
  show("")
  show("/a")
  return 0
}
