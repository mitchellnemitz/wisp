import "env"
import "process"
fn main() -> int {
  env.set("WISP_X", "hello")
  print(unwrap_or(env.get("WISP_X"), "none"))
  print(process.run(["sh", "-c", "printf %s \"$WISP_X\""]))
  return 0
}
