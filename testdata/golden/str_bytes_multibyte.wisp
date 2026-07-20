import "string"
fn main() -> int {
  let s: string = "aébéc"
  print("${length(s)}")
  print("${unwrap_or(string.index_of(s, "é"), -1)}")
  print("${unwrap_or(string.last_index_of(s, "é"), -1)}")
  print("${string.count(s, "é")}")
  print(string.pad_start(s, 9, "."))
  print(string.pad_end(s, 9, "."))
  return 0
}
