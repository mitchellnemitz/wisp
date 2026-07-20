import "process"
fn main() -> int {
  process.exec_command(["cat"])
  return 0
}
