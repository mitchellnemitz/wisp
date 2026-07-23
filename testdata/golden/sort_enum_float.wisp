import "array"
import "string"
enum Ratio: float { Half = 0.5, Full = 1.0 }
fn show(r: Ratio) -> string {
  switch (r) {
    case Ratio.Half { return "half" }
    case Ratio.Full { return "full" }
  }
}
fn main() -> int {
  let xs: Ratio[] = [Ratio.Full, Ratio.Half]
  print(string.join(array.map(array.sort(xs), show), " "))
  return 0
}
