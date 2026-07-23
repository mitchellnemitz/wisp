import "array"
import "string"
enum Size: string { S = "a", M = "b", L = "c" }
fn show(s: Size) -> string {
  switch (s) {
    case Size.S { return "S" }
    case Size.M { return "M" }
    case Size.L { return "L" }
  }
}
fn main() -> int {
  let xs: Size[] = [Size.L, Size.S, Size.M]
  print(string.join(array.map(array.sort(xs), show), " "))
  return 0
}
