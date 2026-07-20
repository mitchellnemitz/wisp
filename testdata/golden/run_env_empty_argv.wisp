import "process"
fn main() -> int {
  let argv: string[] = []
  let e: {string: string} = {"X": "1"}
  let out: string = process.run_env(argv, e)
  print("unreached")
  return 0
}
