fn cleanup() -> void {
  print("cleanup")
}
fn main() -> int {
  on_exit(cleanup)
  return 5
}
