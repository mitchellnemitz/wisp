import "process"
fn main() -> int {
  let p: Process = process.spawn(["echo", "done"])
  let spins: int = 0
  while (!process.is_done(p)) {
    spins = spins + 1
  }
  process.signal(p, "TERM")
  let r: RunResult = process.wait(p)
  print(r.stdout)
  print("code=${r.code}")
  process.signal(p, "TERM")
  print("after-done-noop")
  return 0
}
