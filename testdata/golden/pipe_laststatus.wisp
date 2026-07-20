import "process"
fn main() -> int {
  let a: RunResult = process.pipe([["sh", "-c", "exit 3"], ["cat"]])
  print("a=${a.code}")
  let b: RunResult = process.pipe([["echo", "x"], ["sh", "-c", "exit 5"]])
  print("b=${b.code}")
  return 0
}
