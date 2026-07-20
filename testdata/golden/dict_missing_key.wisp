fn main() -> int {
  let m: {string: int} = { "a": 1 }
  print(to_string(m["nope"]))
  return 0
}
