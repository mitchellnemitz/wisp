import "array"
import "dict"
import "string"
fn show(x: float) -> string { return to_string(x) }
fn main() -> int {
  let m: {float: string} = {}
  m[1.0] = "a"
  m[1.00] = "b"
  print("${unwrap_or(dict.get(m, 1.0), "?")}")
  print("${length(dict.keys(m))}")
  let z: {float: int} = {}
  z[-0.0] = 1
  z[0.0] = 2
  print("${unwrap_or(dict.get(z, -0.0), -1)}")
  print(string.join(array.map(dict.keys(z), show), " "))
  let t: {float: int} = {}
  t[1.00] = 1
  t[2.50] = 2
  print(string.join(array.map(dict.keys(t), show), " "))
  return 0
}
