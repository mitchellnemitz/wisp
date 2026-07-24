import "env"
fn main() -> int {
  print(to_string(length(unwrap_or(env.get("PATH"), "FB")) > 0))
  return 0
}
