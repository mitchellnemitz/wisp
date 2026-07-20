import "process"
fn main() -> int {
  process.exec_command(["false"])
  return 0
}
