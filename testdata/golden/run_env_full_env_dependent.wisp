import "process"
fn main() -> int {
  let e: {string: string} = {"O": "out", "E": "err"}
  let r: RunResult = process.run_env_full(["sh", "-c", "printf %s \"$O\"; printf %s \"$E\" >&2; exit 2"], e)
  print(r.stdout)
  print(r.stderr)
  print(to_string(r.code))
  return 0
}
