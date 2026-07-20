import "env"
fn main() -> int {
  try {
    print(env.get("WISP_GOLDEN_NOPE"))
  } catch (e) {
    print("caught")
  }
  return 0
}
