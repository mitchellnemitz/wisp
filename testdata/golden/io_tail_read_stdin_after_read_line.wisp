import "process"
fn main() -> int {
  read_line()
  let rest: string = read_stdin()
  process.run_status(["printf", "%s", rest])
  return 0
}
