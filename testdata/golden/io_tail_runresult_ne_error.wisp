import "process"
fn main() -> int {
  let r1: RunResult = process.run_full(["true"])
  let r2: RunResult = process.run_full(["true"])
  if (r1 != r2) {
    print("not equal")
  }
  return 0
}
