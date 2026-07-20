import "string"
fn main() -> int {
  let s: string = string.lower("ABC\n")
  print(string.replace(s, "\n", "[nl]"))
  return 0
}
