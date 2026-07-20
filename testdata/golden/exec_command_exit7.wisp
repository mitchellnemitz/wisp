import "process"
fn main() -> int {
  process.exec_command(["sh", "-c", "exit 7"])
  return 0
}
