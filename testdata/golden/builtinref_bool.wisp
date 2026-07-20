import "fs"
fn main() -> int {
  let dir: string = fs.temp_dir()
  let f: string = "${dir}/file"
  fs.write_file(f, "x")
  let lnk: string = "${dir}/lnk"
  fs.symlink(f, lnk)
  let isf: fn(string)->bool = fs.is_file
  let isl: fn(string)->bool = fs.is_symlink
  let isd: fn(string)->bool = fs.is_dir
  let ex: fn(string)->bool = fs.file_exists
  print("${isf(f)}")
  print("${isl(lnk)}")
  print("${isd(dir)}")
  print("${ex(f)}")
  print("${ex("${dir}/missing")}")
  return 0
}
