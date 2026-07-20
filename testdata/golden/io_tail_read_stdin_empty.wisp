import "process"
fn main() -> int {
  let s: string = read_stdin()
  if (s == "") {
    print("(empty)")
  } else {
    process.run_status(["printf", "%s", s])
  }
  return 0
}
