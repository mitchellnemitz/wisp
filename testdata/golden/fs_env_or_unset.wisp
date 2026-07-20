import "env"
fn main() -> int {
  print(env.get_or("WISP_GOLDEN_NOPE", "FB"))
  return 0
}
