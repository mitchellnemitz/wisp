fn classify(x: float) -> string {
  switch (x) {
    case 1.0 { return "one" }
    case 2.5 { return "two-and-half" }
    default { return "other" }
  }
}

fn main() -> int {
  print(classify(1.00))
  print(classify(2.5))
  print(classify(-0.0))
  print(classify(9.0))
  return 0
}
