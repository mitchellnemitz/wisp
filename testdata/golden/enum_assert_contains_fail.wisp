enum Color: int { Red, Green, Blue }

fn main() -> int {
  let cs: Color[] = [Color.Red, Color.Green]
  assert_contains(cs, Color.Blue)
  print("unreached")
  return 0
}
