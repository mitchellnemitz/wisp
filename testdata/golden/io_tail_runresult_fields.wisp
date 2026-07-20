import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["echo", "hi"])
  process.run_status(["printf", "%s", r.stdout])
  print(r.stderr)
  print(to_string(r.code))
  return 0
}
