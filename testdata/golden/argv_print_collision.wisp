import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["print", "hello"])
  print(debug(r))
  return 0
}
