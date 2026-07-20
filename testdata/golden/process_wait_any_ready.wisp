import "process"
fn main() -> int {
  let p: Process = process.spawn(["echo", "y"])
  let s: int = 0
  while (!process.is_done(p)) {
    s = s + 1
  }
  let w: Process = process.wait_any([p], 9)
  let rw: RunResult = process.wait(w)
  print(rw.stdout)
  return 0
}
