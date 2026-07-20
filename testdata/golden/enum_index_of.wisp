import "string"
enum Color { Red, Green, Blue }

fn main() -> int {
  let cs: Color[] = [Color.Red, Color.Green]
  let found: Optional[int] = string.index_of(cs, Color.Green)
  let absent: Optional[int] = string.index_of(cs, Color.Blue)
  print(to_string(is_some(found)))
  print(to_string(unwrap_or(found, -1)))
  print(to_string(is_none(absent)))
  return 0
}
