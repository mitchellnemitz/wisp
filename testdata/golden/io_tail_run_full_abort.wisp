import "process"
fn main() -> int {
  let argv: string[] = []
  let r: RunResult = process.run_full(argv)
  print(r.stdout)
  return 0
}
