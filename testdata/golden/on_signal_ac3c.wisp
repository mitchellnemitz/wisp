import "process"
fn cleanup() -> void {
  print("cleanup")
}
fn shutdown() -> void {
  print("shutting-down")
  exit(0)
}
fn main() -> int {
  on_exit(cleanup)
  on_signal("TERM", shutdown)
  let rc: int = process.run_status(["sh", "-c", "kill -TERM \"$PPID\""])
  print("unreached")
  return 0
}
