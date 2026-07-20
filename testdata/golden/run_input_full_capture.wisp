import "process"
fn main() -> int {
  let r: RunResult = process.run_input_full(["sh", "-c", "cat; printf err >&2; exit 5"], "out")
  print(r.stdout)
  print(r.stderr)
  print(to_string(r.code))
  return 0
}
