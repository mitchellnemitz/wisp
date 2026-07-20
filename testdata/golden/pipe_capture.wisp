import "process"
fn main() -> int {
  let a: RunResult = process.pipe([["printf", "X"], ["cat"]])
  print("a.stdout=${a.stdout}")
  let b: RunResult = process.pipe([["echo", "x"], ["sh", "-c", "cat; echo err >&2"]])
  print("b.stdout=${b.stdout}")
  print("b.stderr=${b.stderr}")
  let c: RunResult = process.pipe([["echo", "x"], ["sh", "-c", "cat >/dev/null; exit 4"]])
  print("c.code=${c.code}")
  return 0
}
