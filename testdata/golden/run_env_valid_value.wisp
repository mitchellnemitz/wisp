import "process"
fn main() -> int {
  let e: {string: string} = {"FOO": "a=b c;d"}
  let out: string = process.run_env(["sh", "-c", "printf %s \"$FOO\""], e)
  print(out)
  return 0
}
