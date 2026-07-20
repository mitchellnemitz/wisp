import "process"
fn main() -> int {
  let p1: Process = process.spawn(["sleep", "2"])
  let p2: Process = process.spawn(["echo", "x"])
  let w: Process = process.wait_any([p1, p2], 0)
  let rw: RunResult = process.wait(w)
  print(rw.stdout)
  let r1: RunResult = process.wait(p1)
  print("done")
  return 0
}
