import "regex"
fn main() -> int {
  print(regex.replace("a1b2", "[0-9]", "#"))
  print(regex.replace("a1b2", "[0-9]+", "#"))
  print(regex.replace("abc", "[0-9]+", "#"))
  return 0
}
