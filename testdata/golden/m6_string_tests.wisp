import "string"
fn main() -> int {
  print("${string.contains("hello", "ell")}")
  print("${string.contains("hello", "")}")
  print("${string.starts_with("hello", "he")}")
  print("${string.ends_with("hello", "lo")}")
  print("${unwrap_or(string.index_of("hello", "l"), -1)}")
  print("${unwrap_or(string.index_of("hello", "z"), -1)}")
  print("${unwrap_or(string.index_of("hello", ""), -1)}")
  return 0
}
