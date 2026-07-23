enum Ratio: float { Half = 0.5, Full = 1.0 }
fn main() -> int {
  let r: Ratio = Ratio.Half
  switch (r) {
    case Ratio.Half { }
  }
  return 0
}
