import "process"
fn main() -> int {
  let empty: string[] = []
  let rc: int = process.run_status(empty)
  print("unreached")
  return 0
}
