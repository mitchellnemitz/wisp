import "dict"
fn main() -> int {
  let m: {bool: int} = {}
  m[true] = 10
  m[false] = 20
  print("${unwrap_or(dict.get(m, true), -1)}")
  print("${unwrap_or(dict.get(m, false), -1)}")
  print("${length(dict.keys(m))}")
  return 0
}
