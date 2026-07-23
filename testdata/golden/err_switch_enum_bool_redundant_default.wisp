enum Flag: bool { On = true, Off = false }
fn main() -> int {
  let f: Flag = Flag.On
  switch (f) {
    case Flag.On { }
    case Flag.Off { }
    default { }
  }
  return 0
}
