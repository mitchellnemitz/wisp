import "process"
fn main() -> int {
  let r1: RunResult = process.run_full(["echo", "hello"])
  process.run_status(["printf", "%s", r1.stdout])
  let r2: RunResult = process.run_full(["sh", "-c", "exit 0"])
  print(to_string(r2.code))
  let r3: RunResult = process.run_full(["sh", "-c", "echo err >&2"])
  process.run_status(["printf", "%s", r3.stderr])
  print(r3.stdout)
  let r4: RunResult = process.run_full(["sh", "-c", "exit 42"])
  print(to_string(r4.code))
  let r5: RunResult = process.run_full(["sh", "-c", "read x; echo got:$x"])
  process.run_status(["printf", "%s", r5.stdout])
  return 0
}
