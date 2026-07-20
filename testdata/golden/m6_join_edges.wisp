import "string"
fn main() -> int {
  let e: string[] = []
  print("[${string.join(e, "-")}]")
  let xs: string[] = ["a", "b", "c"]
  print(string.join(xs, ""))
  print(string.join(xs, ", "))
  return 0
}
