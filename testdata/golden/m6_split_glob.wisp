import "string"
fn main() -> int {
  let parts: string[] = string.split("a*b*c", "*")
  print(string.join(parts, ","))
  let p2: string[] = string.split("x?y", "?")
  print(string.join(p2, ","))
  return 0
}
