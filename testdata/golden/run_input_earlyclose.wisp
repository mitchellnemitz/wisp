import "process"
fn main() -> int {
  print(process.run_input(["head", "-n", "1"], "line1\nline2\nline3\n"))
  return 0
}
