import "fs"
import "process"
fn main() -> int {
  let dir: string = fs.temp_dir()
  let path: string = "${dir}/wisp_fifo"
  process.make_fifo(path)
  let rc: int = process.run_status(["test", "-p", path])
  print("isfifo=${rc}")
  return 0
}
