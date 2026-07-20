fn handler_a() -> void {
  print("a")
}
fn handler_b() -> void {
  print("b")
}
fn main() -> int {
  on_exit(handler_a)
  on_exit(handler_b)
  return 0
}
