fn main() -> int {
  let danger: string = "$(echo PWNED)"
  print(danger)
  print("pre ${danger} post")
  let metas: string = "a;b&c`whoami`"
  print(metas)
  return 0
}
