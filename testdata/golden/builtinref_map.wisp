import "array"
import "fs"
import "string"
fn main() -> int {
  let xs: string[] = ["  a  ", " b "]
  let ts: string[] = array.map(xs, string.trim)
  print(ts[0])
  print(ts[1])
  let us: string[] = array.map(["abc", "de"], string.upper)
  print(us[0])
  print(us[1])
  let dir: string = fs.temp_dir()
  let p: string = "${dir}/exists"
  fs.write_file(p, "x")
  let cand: string[] = [p, "${dir}/missing"]
  let only: string[] = array.filter(cand, fs.is_file)
  print(fs.base_name(only[0]))
  return 0
}
