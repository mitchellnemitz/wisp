struct Box { v: int }
fn main() -> int {
  let b: Box = Box { v: 0 }
  try {
    b.v = 42
    let n: int = to_int("bad")
    b.v = 99
  } catch (e) {
    print("caught")
  }
  print(to_string(b.v))
  return 0
}
