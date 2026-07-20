enum Color { Red, Green, Blue }

fn main() -> int {
  assert_ne(Color.Red, Color.Red)
  print("unreached")
  return 0
}
