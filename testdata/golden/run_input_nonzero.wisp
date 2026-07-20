import "process"
fn main() -> int {
  print(process.run_input(["sh", "-c", "cat; exit 3"], "x"))
  return 0
}
