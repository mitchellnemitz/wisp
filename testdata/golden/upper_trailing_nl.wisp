import "string"
fn main() -> int {
  let s: string = string.upper("abc\n")
  print(string.replace(s, "\n", "[NL]"))
  return 0
}
