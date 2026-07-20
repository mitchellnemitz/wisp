fn main() -> int {
  let e: error = error_with(42, "boom")
  print(e.message)
  print(to_string(e.code))
  return 0
}
