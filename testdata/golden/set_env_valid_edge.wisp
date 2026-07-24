import "env"
fn main() -> int {
  env.set("_OK", "1")
  env.set("A1_B2", "2")
  print(unwrap_or(env.get("_OK"), "none"))
  print(unwrap_or(env.get("A1_B2"), "none"))
  return 0
}
