import "regex"
fn main() -> int {
  let xs: string[] = regex.find_all("x", "[")
  print("unreached")
  return 0
}
