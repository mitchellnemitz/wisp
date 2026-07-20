fn main() -> int {
  for (let i: int = 0; i < 3; i = i + 1) {
    try {
      continue
    } catch (e) {
      print("c")
    }
  }
  return 0
}
