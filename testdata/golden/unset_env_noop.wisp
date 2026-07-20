import "env"
fn main() -> int {
  env.unset("WISP_NEVER_SET")
  print("ok")
  return 0
}
