import "process"
fn main() -> int {
  let e: {string: string} = {"BAD NAME": "x"}
  let r: RunResult = process.run_env_full(["true"], e)
  print("unreached")
  return 0
}
