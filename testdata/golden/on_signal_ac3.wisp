import "process"
fn handler() -> void {
  print("caught")
}
fn handler_term() -> void {
  print("caught-term")
}
fn main() -> int {
  on_signal("USR1", handler)
  on_signal("TERM", handler_term)
  print("before")
  let rc1: int = process.run_status(["sh", "-c", "kill -USR1 \"$PPID\""])
  print("after")
  let rc2: int = process.run_status(["sh", "-c", "kill -TERM \"$PPID\""])
  print("after-term")
  return 0
}
