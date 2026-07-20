import "env"
fn main() -> int {
  let h: bool = env.has("PATH")
  print(to_string(h))
  return 0
}
