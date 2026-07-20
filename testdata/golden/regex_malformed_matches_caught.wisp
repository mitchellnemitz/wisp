import "regex"
fn main() -> int {
  try {
    let m: bool = regex.matches("x", "[")
    print("unreached")
  } catch (e) {
    print("caught")
  }
  return 0
}
