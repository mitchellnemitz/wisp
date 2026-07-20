import "process"
fn main() -> int {
  let s: string[][] = [["echo", "hi"], ["cat"]]
  let r: RunResult = process.pipe(s)
  print(r.stdout)
  return 0
}
