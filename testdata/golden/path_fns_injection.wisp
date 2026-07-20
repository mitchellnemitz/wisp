// Injection (spec P5/AC6): dir_name/base_name treat the argument as INERT data.
// A path containing $(...), backticks, ;, |, *, and spaces renders literally and
// nothing is ever evaluated or globbed -- no pwned* file appears. The captured $0
// is likewise inert (it flows only through a double-quoted expansion); this
// fixture exercises the controllable argument path on all four shells.
import "fs"
fn main() -> int {
  let a: string = "/x/$(touch pwned1)/*"
  print(fs.dir_name(a))
  print(fs.base_name(a))
  let b: string = "/y/`touch pwned2`/z"
  print(fs.dir_name(b))
  print(fs.base_name(b))
  print(fs.base_name("a b ; c | d"))
  print(to_string(fs.file_exists("pwned1")))
  print(to_string(fs.file_exists("pwned2")))
  return 0
}
