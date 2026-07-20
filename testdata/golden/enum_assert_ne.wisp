enum Color { Red, Green, Blue }

fn main() -> int {
  assert_ne(Color.Red, Color.Blue)
  print("passed")
  return 0
}
