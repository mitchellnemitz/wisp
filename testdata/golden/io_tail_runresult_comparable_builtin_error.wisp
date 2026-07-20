import "array"
import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["true"])
  array.unique([r])
  return 0
}
