enum Color { Red, Green, Blue }

fn main() -> int {
  let c: Color = Color.Green
  if (c == Color.Red) {
    print("c==Red")
  }
  if (c == Color.Green) {
    print("c==Green")
  }
  print(to_string(to_int(Color.Blue)))
  return 0
}
