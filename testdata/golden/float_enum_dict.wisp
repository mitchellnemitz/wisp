import "dict"
enum Ratio: float { Half = 0.5, Full = 1.0 }

fn main() -> int {
  let m: {Ratio: int} = {}
  m[Ratio.Half] = 1
  m[Ratio.Full] = 2
  print("${unwrap_or(dict.get(m, Ratio.Half), -1)}")
  print("${length(dict.keys(m))}")
  return 0
}
