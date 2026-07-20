import "process"
fn main() -> int {
  let s: string = read_stdin()
  process.run_status(["printf", "%s", s])
  return 0
}
