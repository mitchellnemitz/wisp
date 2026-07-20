import "process"
fn main() -> int {
  let e: {string: string} = {"CODE": "3"}
  let rc: int = process.run_env_status(["sh", "-c", "exit \"$CODE\""], e)
  print(to_string(rc))
  return 0
}
