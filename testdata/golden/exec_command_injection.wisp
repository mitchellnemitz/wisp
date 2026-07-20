import "process"
fn main() -> int {
  process.exec_command(["printf", "%s", "$(touch PWNED); `id`; *"])
  return 0
}
