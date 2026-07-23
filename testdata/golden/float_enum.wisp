enum Ratio: float { Half = 0.5, Full = 1.0 }

fn main() -> int {
  let r: Ratio = Ratio.Half
  print("${to_float(r)}")
  switch (r) {
    case Ratio.Half { print("half") }
    case Ratio.Full { print("full") }
  }
  return 0
}
