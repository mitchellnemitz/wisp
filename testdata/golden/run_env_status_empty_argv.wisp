import "process"
fn main() -> int {
  let e: {string: string} = {"X": "1"}
  let argv: string[] = []
  let rc: int = process.run_env_status(argv, e)
  print("unreached")
  return 0
}
