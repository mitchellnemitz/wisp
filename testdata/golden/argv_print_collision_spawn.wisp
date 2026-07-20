import "process"
fn main() -> int {
  let p: Process = process.spawn(["print", "hello"])
  let r: RunResult = process.wait(p)
  print(debug(r))
  return 0
}
