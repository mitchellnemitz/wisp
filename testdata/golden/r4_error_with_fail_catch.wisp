fn main() -> int {
  try {
    let n: int = to_int("bad")
    print(to_string(n))
  } catch (e) {
    print(to_string(e.code))
  }
  return 0
}
