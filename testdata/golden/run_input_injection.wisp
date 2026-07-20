import "process"
fn main() -> int {
  print(process.run_input(["cat"], "$(touch PWNED); `id`; *"))
  return 0
}
