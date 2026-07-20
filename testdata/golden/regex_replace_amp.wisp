import "regex"
fn main() -> int {
  print(regex.replace("ab", "(a)(b)", "<&>"))
  print(regex.replace("a&b", "&", "\\&"))
  return 0
}
