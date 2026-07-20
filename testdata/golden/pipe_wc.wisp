import "process"
import "string"
fn main() -> int {
  let r: RunResult = process.pipe([["printf", "a\nb\nc\n"], ["wc", "-l"]])
  print(string.trim(r.stdout))
  print("code=${r.code}")
  return 0
}
