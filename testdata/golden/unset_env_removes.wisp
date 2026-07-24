import "env"
fn main() -> int {
  env.set("WISP_X", "hello")
  env.unset("WISP_X")
  print(unwrap_or(env.get("WISP_X"), "none"))
  return 0
}
