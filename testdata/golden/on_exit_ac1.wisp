fn done() -> void {
  print("bye")
}
fn main() -> int {
  on_exit(done)
  print("main")
  return 0
}
