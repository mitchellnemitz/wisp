import "process"
fn main() -> int {
  let r: RunResult = process.pipe([["sh", "-c", "if read x; then echo \"got=$x\"; else echo empty; fi"], ["cat"]])
  print(r.stdout)
  return 0
}
