fn classify(n: int) -> string {
  switch (n) {
    case 007 {
      return "seven"
    }
    default {
      return "other"
    }
  }
}

fn main() -> int {
  print(classify(7))
  print(classify(5))
  return 0
}
