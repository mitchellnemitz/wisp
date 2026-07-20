import "process"
fn main() -> int {
  print(process.run(["cat", "no_such_file_xyz"]))
  return 0
}
