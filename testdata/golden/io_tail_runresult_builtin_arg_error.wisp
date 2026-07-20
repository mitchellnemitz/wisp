import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["true"])
  print(r)
  return 0
}
