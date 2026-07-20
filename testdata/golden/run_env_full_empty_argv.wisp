import "process"
fn main() -> int {
  let e: {string: string} = {"X": "1"}
  let argv: string[] = []
  let r: RunResult = process.run_env_full(argv, e)
  print("unreached")
  return 0
}
