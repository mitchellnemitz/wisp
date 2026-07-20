import "fs"
import "process"
fn main() -> int {
  let dir: string = fs.temp_dir()
  let path: string = "${dir}/$(touch PWNED2)fifo"
  process.make_fifo(path)
  let isfifo: int = process.run_status(["test", "-p", path])
  print("isfifo=${isfifo}")
  let pwned: bool = fs.file_exists("PWNED2")
  print("pwned=${pwned}")
  return 0
}
