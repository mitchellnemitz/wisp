import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["echo", "hi"])
  print(debug(r))
  return 0
}
