import "process"
fn main() -> int {
  let p: Process = process.spawn(["echo", "hi"])
  let r1: RunResult = process.wait(p)
  let r2: RunResult = process.wait(p)
  print(r1.stdout)
  print(r2.stdout)
  print("${r1.code}-${r2.code}")
  return 0
}
