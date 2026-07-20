import "process"
fn main() -> int {
  let p: Process = process.spawn(["sleep", "5"])
  process.signal(p, "KILL")
  let r: RunResult = process.wait(p)
  if (r.code != 0) {
    print("killed")
  } else {
    print("survived")
  }
  return 0
}
