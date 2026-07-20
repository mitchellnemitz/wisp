fn main() -> int {
  let x: int = 42
  let s: string = debug(x)
  print(s, stderr)
  let combined: string = "val=" + debug(x)
  print(combined)
  return 0
}
