enum Color { Red, Green, Blue }

fn main() -> int {
  let cs: Color[] = [Color.Red, Color.Green]
  assert_contains(cs, Color.Red)
  print("passed")
  return 0
}
