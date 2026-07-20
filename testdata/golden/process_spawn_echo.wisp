import "process"
fn main() -> int {
  let p: Process = process.spawn(["echo", "hi"])
  let r: RunResult = process.wait(p)
  print(r.stdout)
  print("code=${r.code}")
  return 0
}
