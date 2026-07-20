import "regex"
fn main() -> int {
  let r: string = regex.replace("x", "[", "y")
  print("unreached")
  return 0
}
