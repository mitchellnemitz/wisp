import "array"
import "string"
fn show(b: bool) -> string { return to_string(b) }
fn main() -> int {
  let xs: bool[] = [true, false, true, false]
  print(string.join(array.map(array.sort(xs), show), " "))
  return 0
}
