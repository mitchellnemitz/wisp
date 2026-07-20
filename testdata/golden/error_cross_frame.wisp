fn parse(s: string) -> int {
  return to_int(s)
}
fn main() -> int {
  try {
    let n: int = parse("nope")
    print("unreached")
  } catch (e) {
    print("caught")
  } finally {
    print("cleanup")
  }
  return 0
}
