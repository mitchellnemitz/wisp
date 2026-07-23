import "dict"
fn main() -> int {
  let m: {bool: int} = {}
  m[true] = 1
  m[false] = 2
  print("${dict.get_or(m, true, -1)}")
  return 0
}
