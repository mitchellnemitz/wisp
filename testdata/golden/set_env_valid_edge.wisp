import "env"
fn main() -> int {
  env.set("_OK", "1")
  env.set("A1_B2", "2")
  print(env.get_or("_OK", "none"))
  print(env.get_or("A1_B2", "none"))
  return 0
}
