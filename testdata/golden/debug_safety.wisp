fn main() -> int {
  let dangerous: string = "$(touch pwned)"
  print(debug(dangerous))
  let xs: string[] = ["$(touch pwned)"]
  print(debug(xs))
  let e: error = error("$(touch pwned)")
  print(debug(e))
  return 0
}
