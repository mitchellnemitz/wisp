import "regex"
fn main() -> int {
  try {
    let r: string = regex.replace("x", "[", "y")
    print("unreached")
  } catch (e) {
    print("caught")
  }
  return 0
}
