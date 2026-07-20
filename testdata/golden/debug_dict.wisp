fn main() -> int {
  let m: {string: int} = { "a": 1, "b": 2 }
  print(debug(m))
  let empty: {string: int} = {}
  print(debug(empty))
  return 0
}
