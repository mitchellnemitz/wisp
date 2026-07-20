import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["print", "hello"])
  print(to_string(r.code))
  return 0
}
