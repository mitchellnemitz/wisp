import "process"
fn main() -> int {
  let e: {string: string} = {"FOO\nBAR": "x"}
  let out: string = process.run_env(["true"], e)
  print("unreached")
  return 0
}
