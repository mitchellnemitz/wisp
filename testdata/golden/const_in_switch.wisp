const HOUR: int = 60 * 60
const GREET: string = "hello"

fn classify(n: int) -> string {
  switch (n) {
    case HOUR {
      return "one hour"
    }
    case 60 * 2 {
      return "two minutes"
    }
    default {
      return "other"
    }
  }
}

fn main() -> int {
  print(GREET)
  print(classify(3600))
  print(classify(120))
  print(classify(1))
  return 0
}
