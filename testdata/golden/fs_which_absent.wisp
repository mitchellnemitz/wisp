import "fs"
fn main() -> int {
  let p: Optional[string] = fs.which("wisp_definitely_not_a_command_xyz")
  print(to_string(is_none(p)))
  return 0
}
