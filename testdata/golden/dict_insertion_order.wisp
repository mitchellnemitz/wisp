fn main() -> int {
  let m: {string: int} = {}
  m["zebra"] = 1
  m["apple"] = 2
  m["mango"] = 3
  m["apple"] = 20
  for (k in m) { print("${k}=${m[k]}") }
  return 0
}
