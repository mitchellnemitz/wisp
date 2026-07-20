import "env"
import "process"
fn main() -> int {
  env.set("WISP_X", "hello")
  print(env.get_or("WISP_X", "none"))
  print(process.run(["sh", "-c", "printf %s \"$WISP_X\""]))
  return 0
}
