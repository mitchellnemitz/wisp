import "string"
fn main() -> int {
  let parts: string[] = string.split("one,two,three", ",")
  print("${length(parts)}")
  print(string.join(parts, "/"))
  return 0
}
