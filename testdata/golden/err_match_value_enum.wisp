enum Color: int { Red, Green }
fn main() -> int {
  let c: Color = Color.Red
  match (c) {
    case Red { print("r") }
    case Green { print("g") }
  }
  return 0
}
