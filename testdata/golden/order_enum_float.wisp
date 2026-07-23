enum Ratio: float { Half = 0.5, Full = 1.0 }
fn main() -> int {
  print(to_string(Ratio.Half < Ratio.Full))
  print(to_string(Ratio.Full < Ratio.Half))
  print(to_string(Ratio.Half <= Ratio.Half))
  print(to_string(Ratio.Full > Ratio.Half))
  print(to_string(Ratio.Half >= Ratio.Full))
  return 0
}
