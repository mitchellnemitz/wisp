fn classify(x: int) -> string {
  if (x > 10) {
    return "big"
  } else if (x > 0) {
    return "small"
  } else {
    return "nonpositive"
  }
}
fn main() -> int {
  print(classify(50))
  print(classify(5))
  print(classify(-2))
  return 0
}
