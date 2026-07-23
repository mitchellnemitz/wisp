enum Flag: bool { Off = false, On = true }
fn main() -> int {
  print(to_string(Flag.Off < Flag.On))
  print(to_string(Flag.On < Flag.Off))
  print(to_string(Flag.Off <= Flag.Off))
  print(to_string(Flag.On > Flag.Off))
  print(to_string(Flag.Off >= Flag.On))
  return 0
}
