import "process"
fn main() -> int {
  let empty: string[] = []
  let a: RunResult = process.pipe([empty, ["cat"]])
  print("a.code=${a.code} a.out=[${a.stdout}]")
  let b: RunResult = process.pipe([["echo", "x"], empty])
  print("b.code=${b.code}")
  return 0
}
