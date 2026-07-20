enum Color { Red, Green, Blue }

fn name(c: Color) -> string {
  switch (c) {
    case Color.Red { return "red" }
    case Color.Green { return "green" }
    case Color.Blue { return "blue" }
  }
  return "unreachable"
}

fn main() -> int {
  print(name(Color.Red))
  print(name(Color.Green))
  print(name(Color.Blue))
  return 0
}
