import "process"
fn main() -> int {
  let override_result: string = process.run_env(["sh", "-c", "printf %s \"$FOO\""], {"FOO": "inner"})
  print(override_result)
  let e: {string: string} = {"FOO": "inner"}
  let augment_result: string = process.run_env(["sh", "-c", "test -n \"$HOME\" && printf found || printf missing"], e)
  print(augment_result)
  let empty_e: {string: string} = {}
  let empty_result: string = process.run_env(["sh", "-c", "printf %s hi"], empty_e)
  print(empty_result)
  return 0
}
