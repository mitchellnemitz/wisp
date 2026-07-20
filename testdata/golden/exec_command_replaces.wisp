import "process"
fn main() -> int {
  process.exec_command(["echo", "hi"])
  print("after")
  return 0
}
