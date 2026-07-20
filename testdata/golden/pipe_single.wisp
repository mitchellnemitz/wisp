import "process"
fn main() -> int {
  let r: RunResult = process.pipe([["echo", "solo"]])
  print(r.stdout)
  print("code=${r.code}")
  return 0
}
