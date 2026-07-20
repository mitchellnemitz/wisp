import "fs"
import "process"
fn main() -> int {
  let dir: string = fs.temp_dir()
  let path: string = "${dir}/wisp_fifo2"
  process.make_fifo(path)
  process.make_fifo(path)
  return 0
}
