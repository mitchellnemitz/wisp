import "math"
enum Size: string { S = "a", M = "b", L = "c" }
fn show(s: Size) -> string {
  switch (s) {
    case Size.S { return "S" }
    case Size.M { return "M" }
    case Size.L { return "L" }
  }
}
fn main() -> int {
  print(show(math.min(Size.L, Size.S)))
  print(show(math.max(Size.L, Size.S)))
  return 0
}
