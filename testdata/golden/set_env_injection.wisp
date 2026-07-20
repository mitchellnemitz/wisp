import "env"
fn main() -> int {
  env.set("WISP_INJ", "$(touch PWNED); `id`; *")
  print(env.get_or("WISP_INJ", "none"))
  return 0
}
