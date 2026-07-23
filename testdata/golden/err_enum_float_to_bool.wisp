enum Ratio: float { Half = 0.5 }
fn main() -> int {
  let r: Ratio = Ratio.Half
  let b: bool = to_bool(r)
  return 0
}
