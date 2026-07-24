import "env"
fn main() -> int {
  print(to_string(is_none(env.get("WISP_GOLDEN_NOPE"))))
  return 0
}
