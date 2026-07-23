import "math"
enum Ratio: float { Half = 0.5, Full = 1.0 }
fn show(r: Ratio) -> string {
  switch (r) {
    case Ratio.Half { return "half" }
    case Ratio.Full { return "full" }
  }
}
fn main() -> int {
  print(show(math.min(Ratio.Full, Ratio.Half)))
  print(show(math.max(Ratio.Full, Ratio.Half)))
  return 0
}
