enum Color: int { Red = 5, Green, Blue }

fn main() -> int {
  print(to_string(to_int(Color.Green)))
  print(to_string(Color.Red == Color.Green))
  print(to_string(Color.Red == Color.Red))
  return 0
}
