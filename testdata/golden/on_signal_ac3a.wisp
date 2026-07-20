import "process"
fn shutdown() -> void {
  exit(3)
}
fn main() -> int {
  on_signal("TERM", shutdown)
  print("before")
  let rc: int = process.run_status(["sh", "-c", "kill -TERM \"$PPID\""])
  print("unreached")
  return 0
}
