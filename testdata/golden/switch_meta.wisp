fn classify(s: string) -> string {
  switch (s) {
    case "*" {
      return "star"
    }
    case "[a]" {
      return "bracket"
    }
    case "a|b" {
      return "pipe"
    }
    case "?" {
      return "question"
    }
    default {
      return "other"
    }
  }
}
fn main() -> int {
  print(classify("*"))
  print(classify("[a]"))
  print(classify("a|b"))
  print(classify("?"))
  print(classify("x"))
  print(classify("starbucks"))
  return 0
}
