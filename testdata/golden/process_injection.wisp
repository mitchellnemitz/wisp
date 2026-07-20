import "fs"
import "process"
fn main() -> int {
  let p: Process = process.spawn(["printf", "%s", "$(touch PWNED); `touch PWNED`; a;b *"])
  let r: RunResult = process.wait(p)
  print(r.stdout)
  let pwned: bool = fs.file_exists("PWNED")
  print("pwned=${pwned}")
  return 0
}
