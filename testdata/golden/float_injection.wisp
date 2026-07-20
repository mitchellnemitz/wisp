fn main() -> int {
  let f: float = to_float("1; system(\"touch /tmp/wisp_pwn_golden\")")
  print(to_string(f))
  return 0
}
