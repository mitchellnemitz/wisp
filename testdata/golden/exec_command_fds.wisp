import "process"
fn main() -> int {
  process.exec_command(["sh", "-c", "printf out; printf err >&2"])
  return 0
}
