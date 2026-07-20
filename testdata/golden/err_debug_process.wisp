import "process"
fn main() -> int {
  let p: Process = process.spawn(["true"])
  let _ : string = debug(p)
  return 0
}
