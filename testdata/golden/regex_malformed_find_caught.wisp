import "regex"
fn main() -> int {
  try {
    let p: Optional[string] = regex.find("x", "[")
    print("unreached")
  } catch (e) {
    print("caught")
  }
  return 0
}
