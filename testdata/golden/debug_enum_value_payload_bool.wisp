enum Flag: bool { Yes = true, No = false }
enum Box { B(Flag) }
fn main() -> int {
  print(debug(Box.B(Flag.Yes)))
  return 0
}
