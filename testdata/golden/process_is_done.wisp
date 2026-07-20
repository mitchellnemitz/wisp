import "process"
fn main() -> int {
  let p: Process = process.spawn(["sleep", "1"])
  print("${process.is_done(p)}")
  let r: RunResult = process.wait(p)
  print("${process.is_done(p)}")
  return 0
}
