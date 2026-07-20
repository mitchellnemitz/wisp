import "process"
fn main() -> int {
  let rc: int = process.run_status(["true"])
  print(to_string(rc))
  return 0
}
