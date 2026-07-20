import "process"
fn main() -> int {
  print(process.run_input(["tr", "a-z", "A-Z"], "hi"))
  return 0
}
