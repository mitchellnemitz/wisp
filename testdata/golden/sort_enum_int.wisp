import "array"
import "string"
enum Color: int { Red, Green, Blue }
fn show(c: Color) -> string {
  switch (c) {
    case Color.Red { return "R" }
    case Color.Green { return "G" }
    case Color.Blue { return "B" }
  }
}
fn main() -> int {
  let xs: Color[] = [Color.Blue, Color.Red, Color.Green]
  print(string.join(array.map(array.sort(xs), show), " "))
  return 0
}
