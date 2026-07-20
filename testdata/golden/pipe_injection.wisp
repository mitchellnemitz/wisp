import "fs"
import "process"
fn main() -> int {
  let r: RunResult = process.pipe([["printf", "%s", "$(touch PWNED); `touch PWNED`; a|b *"], ["cat"]])
  print(r.stdout)
  let pwned: bool = fs.file_exists("PWNED")
  print("pwned=${pwned}")
  return 0
}
