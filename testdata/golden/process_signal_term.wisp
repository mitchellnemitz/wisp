import "process"
fn main() -> int {
  let p: Process = process.spawn(["sleep", "5"])
  process.signal(p, "TERM")
  let r: RunResult = process.wait(p)
  if (r.code != 0) {
    print("terminated")
  } else {
    print("survived")
  }
  return 0
}
