import "array"
import "string"
enum Flag: bool { Off = false, On = true }
fn show(f: Flag) -> string {
  switch (f) {
    case Flag.Off { return "off" }
    case Flag.On { return "on" }
  }
}
fn main() -> int {
  let xs: Flag[] = [Flag.On, Flag.Off, Flag.On]
  print(string.join(array.map(array.sort(xs), show), " "))
  return 0
}
