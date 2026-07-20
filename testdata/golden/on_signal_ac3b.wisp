import "process"
fn handler_a() -> void {
  print("a")
}
fn handler_b() -> void {
  print("b")
}
fn main() -> int {
  on_signal("USR1", handler_a)
  on_signal("USR1", handler_b)
  let rc: int = process.run_status(["sh", "-c", "kill -USR1 \"$PPID\""])
  return 0
}
