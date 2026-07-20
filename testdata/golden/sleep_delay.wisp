fn main() -> int {
  let t0: int = now()
  sleep(1)
  let t1: int = now()
  print(to_string(t1 - t0 >= 1))
  return 0
}
