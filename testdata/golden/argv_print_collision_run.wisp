import "process"
fn main() -> int {
  let s: string = process.run(["print", "hello"])
  print(s)
  return 0
}
