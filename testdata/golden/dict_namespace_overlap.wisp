import "dict"
fn main() -> int {
  let m: {string: int} = {}
  m["len"] = 111
  m["0"] = 222
  m["keys"] = 333
  print("len=${m["len"]}")
  print("z=${m["0"]}")
  print("keys=${m["keys"]}")
  print("n=${length(dict.keys(m))}")
  return 0
}
