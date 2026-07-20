import "env"
fn main() -> int {
  print(to_string(length(env.get_or("PATH", "FB")) > 0))
  return 0
}
