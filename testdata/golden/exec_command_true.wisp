import "process"
fn main() -> int {
  process.exec_command(["true"])
  return 0
}
