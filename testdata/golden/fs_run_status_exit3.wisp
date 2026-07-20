import "process"
fn main() -> int {
  let rc: int = process.run_status(["sh", "-c", "exit 3"])
  print(to_string(rc))
  return 0
}
