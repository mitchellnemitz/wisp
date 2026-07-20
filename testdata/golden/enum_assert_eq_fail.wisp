enum Color { Red, Green, Blue }

fn main() -> int {
  assert_eq(Color.Red, Color.Blue)
  print("unreached")
  return 0
}
