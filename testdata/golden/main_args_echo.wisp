fn main(args: string[]) -> int {
  print("argc=${length(args)}")
  for (a in args) {
    print(a)
  }
  return 0
}
