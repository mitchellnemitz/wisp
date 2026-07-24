import "dict"
fn main() -> int {
  let m: {bool: int} = {}
  m[true] = 1
  m[false] = 2
  print("${unwrap_or(dict.get(m, true), -1)}")
  return 0
}
