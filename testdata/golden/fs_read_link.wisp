import "fs"
fn main() -> int {
  let t: string = "target_file.txt"
  fs.write_file(t, "hello")
  let s: string = "the_link"
  fs.symlink(t, s)
  let r: Optional[string] = fs.read_link(s)
  match (r) {
    case Some(v) { print(v) }
    case None { print("none") }
  }
  let r2: Optional[string] = fs.read_link(t)
  print(to_string(is_none(r2)))
  let r3: Optional[string] = fs.read_link("missing_path_xyz")
  print(to_string(is_none(r3)))
  return 0
}
