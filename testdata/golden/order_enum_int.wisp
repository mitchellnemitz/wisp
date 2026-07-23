enum Color: int { Red, Green, Blue }
fn main() -> int {
  print(to_string(Color.Red < Color.Green))
  print(to_string(Color.Blue < Color.Red))
  print(to_string(Color.Green <= Color.Green))
  print(to_string(Color.Blue > Color.Red))
  print(to_string(Color.Red >= Color.Blue))
  return 0
}
