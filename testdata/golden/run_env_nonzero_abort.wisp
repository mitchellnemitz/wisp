import "process"
fn main() -> int {
  let e: {string: string} = {"X": "1"}
  let out: string = process.run_env(["sh", "-c", "exit 4"], e)
  print("unreached")
  return 0
}
