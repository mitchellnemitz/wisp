import "process"
fn main() -> int {
  let e: {string: string} = {"BAD NAME": "x"}
  let rc: int = process.run_env_status(["true"], e)
  print("unreached")
  return 0
}
