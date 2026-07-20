fn handle(code: int) -> string {
  switch (code) {
    case 0 {
      return "ok"
    }
    case 1, 2 {
      return "retry"
    }
    default {
      return "fail"
    }
  }
}
fn main() -> int {
  print(handle(0))
  print(handle(1))
  print(handle(2))
  print(handle(9))
  return 0
}
