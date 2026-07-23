import "dict"
fn main() -> int {
  let m: {bool: int} = {}
  m[true] = 10
  m[false] = 20
  print("${dict.get_or(m, true, -1)}")
  print("${dict.get_or(m, false, -1)}")
  print("${length(dict.keys(m))}")
  return 0
}
