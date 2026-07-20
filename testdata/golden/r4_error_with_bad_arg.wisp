fn main() -> int {
  let e: error = error_with("msg", 42)
  print(e.message)
  return 0
}
