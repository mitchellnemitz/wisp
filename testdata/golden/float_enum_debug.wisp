enum Ratio: float { Half = 0.5, Full = 1.0 }
enum Wrap { V(Ratio) }

fn main() -> int {
  let w: Wrap = Wrap.V(Ratio.Half)
  print(debug(w))
  return 0
}
