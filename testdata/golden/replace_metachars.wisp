import "string"
fn main() -> int {
  print(string.replace("x*y*z", "*", "#"))
  print(string.replace("a[b]c", "[b]", "B"))
  print(string.replace("p?q", "?", "Q"))
  return 0
}
