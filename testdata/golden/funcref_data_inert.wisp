fn main() -> int {
  let cmd: string = "$(touch SENTINEL); `id`"
  print("data=${cmd}")
  return 0
}
