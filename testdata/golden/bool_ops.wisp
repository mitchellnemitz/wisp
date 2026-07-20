fn main() -> int {
  let t: bool = true
  let f: bool = false
  print("${t && t}")
  print("${t && f}")
  print("${f || t}")
  print("${f || f}")
  print("${!t}")
  print("${!f}")
  return 0
}
