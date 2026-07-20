import "process"
fn main() -> int {
  let rc: int = process.run_status(["false"])
  print(to_string(rc))
  print("after")
  return 0
}
