import "array"
enum Color: int { Red = 5, Green, Blue }

fn main() -> int {
  print(to_string(to_int(Color.Green)))
  print(to_string(Color.Red == Color.Green))
  print(to_string(Color.Red == Color.Red))
  let colors: Color[] = [Color.Red, Color.Blue]
  print(to_string(array.contains(colors, Color.Green)))
  print(to_string(array.contains(colors, Color.Red)))
  print(to_string(unwrap_or(array.index_of(colors, Color.Blue), -1)))
  print(to_string(length(array.unique(colors))))
  return 0
}
