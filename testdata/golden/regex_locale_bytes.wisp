import "regex"
fn main() -> int {
  print(to_string(length(regex.find_all("aéb", "."))))
  print(regex.replace("é", ".", "_"))
  return 0
}
