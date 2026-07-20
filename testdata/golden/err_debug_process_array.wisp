import "process"
fn main() -> int {
  let p: Process = process.spawn(["true"])
  let ps: Process[] = [p]
  let _ : string = debug(ps)
  return 0
}
