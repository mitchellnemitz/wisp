fn cleanup() -> void {
  print("cleanup")
}
fn main() -> int {
  on_exit(cleanup)
  let z: int = 0
  let x: int = 1 / z
  return 0
}
