import "process"
fn main() -> int {
  print(to_string(process.pid_alive(2147483647)))
  return 0
}
