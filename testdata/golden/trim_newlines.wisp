import "string"
fn main() -> int {
  let s: string = string.trim("\n\t hi there \n\n")
  print(string.replace(s, " ", "_"))
  return 0
}
