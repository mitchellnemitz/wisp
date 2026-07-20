import "env"
fn main() -> int {
  env.set("WISP_X", "hello")
  env.unset("WISP_X")
  print(env.get_or("WISP_X", "none"))
  return 0
}
