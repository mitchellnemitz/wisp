import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["echo", "hi"])
  print(r.message)
  return 0
}
