import "process"
fn main() -> int {
  print(to_string(process.pid_alive(0)))
  return 0
}
