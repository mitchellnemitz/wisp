import "process"
fn main() -> int {
  let p: Process = process.spawn(["sh", "-c", "printf out; printf err >&2; exit 2"])
  let r: RunResult = process.wait(p)
  print(r.stdout)
  print(r.stderr)
  print("code=${r.code}")
  return 0
}
