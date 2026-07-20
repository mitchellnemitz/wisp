import "process"
fn main() -> int {
  let r: RunResult = process.pipe([["echo", "hello"], ["tr", "a-z", "A-Z"]])
  print(r.stdout)
  print("code=${r.code}")
  return 0
}
