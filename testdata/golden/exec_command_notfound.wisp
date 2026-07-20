import "process"
fn main() -> int {
  process.exec_command(["wisp-no-such-cmd-xyzzy"])
  return 0
}
