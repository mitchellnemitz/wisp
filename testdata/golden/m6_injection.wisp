import "string"
fn main() -> int {
  let danger: string = "$(echo PWNED);ls"
  let parts: string[] = string.split(danger, ";")
  print(parts[0])
  print(parts[1])
  print(string.join(parts, "|"))
  print("${string.contains(danger, "PWNED")}")
  print("${unwrap_or(string.index_of(danger, "ls"), -1)}")
  print(string.repeat(danger, 2))
  print("${string.starts_with(danger, "$(echo")}")
  print("${string.ends_with(danger, "ls")}")
  return 0
}
