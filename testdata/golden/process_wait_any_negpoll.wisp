import "process"
fn main() -> int {
  let p: Process = process.spawn(["echo", "x"])
  let poll: int = -1
  let w: Process = process.wait_any([p], poll)
  return 0
}
