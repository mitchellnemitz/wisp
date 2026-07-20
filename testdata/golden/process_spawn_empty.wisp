import "process"
fn main() -> int {
  let argv: string[] = []
  let p: Process = process.spawn(argv)
  return 0
}
