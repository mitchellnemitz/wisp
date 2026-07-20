import "regex"
fn main() -> int {
  try {
    let xs: string[] = regex.find_all("x", "[")
    print("unreached")
  } catch (e) {
    print("caught")
  }
  return 0
}
