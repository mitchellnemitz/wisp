enum Color: int { Red = 5, Green, Blue }

fn main() -> int {
  let b: bool = false
  if (b || Color.Red) {
    return 1
  }
  return 0
}
