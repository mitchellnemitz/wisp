fn bail() -> void {
  exit(5)
}
fn main() -> int {
  try {
    print("in try")
    bail()
    print("unreached")
  } catch (e) {
    print("catch")
  } finally {
    print("finally should not run")
  }
  return 0
}
