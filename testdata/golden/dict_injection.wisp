fn main() -> int {
  let m: {string: int} = {}
  m["$(echo PWN); `id`; rm -rf x"] = 42
  print("v=${m["$(echo PWN); `id`; rm -rf x"]}")
  for (k in m) { print("k=${k}") }
  return 0
}
