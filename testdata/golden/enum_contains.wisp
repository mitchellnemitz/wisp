import "string"
enum Color: int { Red, Green, Blue }

fn main() -> int {
  let cs: Color[] = [Color.Red, Color.Green]
  print(to_string(string.contains(cs, Color.Green)))
  print(to_string(string.contains(cs, Color.Blue)))
  return 0
}
