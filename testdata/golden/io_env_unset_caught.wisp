import "env"
fn main() -> int {
  print(unwrap_or(env.get("WISP_GOLDEN_NOPE"), "caught"))
  return 0
}
