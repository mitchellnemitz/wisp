import "process"
fn main() -> int {
  let e: string[] = []
  let r: RunResult = process.run_input_full(e, "x")
  print(r.stdout)
  return 0
}
