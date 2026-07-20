fn main() -> int {
  let a: int = 7
  let b: int = 0
  try {
    print(to_string(a % b))
  } catch (e) {
    print(e.message)
  }
  return 0
}
