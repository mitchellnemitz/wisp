enum Color: int { Red, Green, Blue }

fn eq2[T: comparable](a: T, b: T) -> bool {
  return a == b
}

fn main() -> int {
  print(to_string(eq2(Color.Red, Color.Red)))
  print(to_string(eq2(Color.Red, Color.Blue)))
  return 0
}
