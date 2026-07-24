import "env"
fn main() -> int {
  env.set("WISP_INJ", "$(touch PWNED); `id`; *")
  print(unwrap_or(env.get("WISP_INJ"), "none"))
  return 0
}
