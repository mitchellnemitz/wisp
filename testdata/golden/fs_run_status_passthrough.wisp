import "process"
fn main() -> int {
  let rc: int = process.run_status(["sh", "-c", "echo child-out; echo child-err 1>&2; exit 0"])
  print(to_string(rc))
  return 0
}
