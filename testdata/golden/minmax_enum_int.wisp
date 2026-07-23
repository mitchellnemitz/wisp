import "math"
enum Color: int { Red, Green, Blue }
fn show(c: Color) -> string {
  switch (c) {
    case Color.Red { return "R" }
    case Color.Green { return "G" }
    case Color.Blue { return "B" }
  }
}
fn main() -> int {
  print(show(math.min(Color.Blue, Color.Red)))
  print(show(math.max(Color.Blue, Color.Red)))
  return 0
}
