import "process"
fn main() -> int {
  let ps: Process[] = []
  let w: Process = process.wait_any(ps, 1)
  return 0
}
