import "process"
fn main() -> int {
  let e: {string: string} = {"1FOO": "x"}
  let out: string = process.run_env(["true"], e)
  print("unreached")
  return 0
}
