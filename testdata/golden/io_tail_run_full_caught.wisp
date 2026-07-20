import "process"
fn main() -> int {
  let argv: string[] = []
  try {
    process.run_full(argv)
  } catch (e) {
    print("caught: run_full")
  }
  return 0
}
