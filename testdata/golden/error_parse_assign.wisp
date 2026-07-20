fn main() -> int {
  let n: int = 0
  try {
    n = to_int("bad")
  } catch (e) {
    n = -1
  }
  print(to_string(n))
  return 0
}
