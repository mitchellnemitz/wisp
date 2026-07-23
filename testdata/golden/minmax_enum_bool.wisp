import "math"
enum Flag: bool { Off = false, On = true }
fn show(f: Flag) -> string {
  switch (f) {
    case Flag.Off { return "off" }
    case Flag.On { return "on" }
  }
}
fn main() -> int {
  print(show(math.min(Flag.On, Flag.Off)))
  print(show(math.max(Flag.On, Flag.Off)))
  return 0
}
