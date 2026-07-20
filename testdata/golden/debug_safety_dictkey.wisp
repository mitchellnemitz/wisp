fn main() -> int {
  let m: {string: int} = {}
  m["$(touch pwned_key)"] = 1
  print(debug(m))
  return 0
}
