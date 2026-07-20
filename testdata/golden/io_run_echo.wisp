import "process"
fn main() -> int {
  print(process.run(["echo", "hi"]))
  return 0
}
