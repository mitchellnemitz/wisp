import "process"
fn main() -> int {
  let e: {string: string} = {"WISP_TEST_VAR": "hello"}
  let out: string = process.run_env(["sh", "-c", "printf %s \"$WISP_TEST_VAR\""], e)
  print(out)
  return 0
}
