import "process"
import "string"
fn main() -> int {
  print(string.trim(process.run_input(["wc", "-c"], "abc")))
  return 0
}
