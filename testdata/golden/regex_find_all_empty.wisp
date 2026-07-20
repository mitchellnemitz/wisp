import "regex"
fn main() -> int {
  print(to_string(length(regex.find_all("abc", "[0-9]+"))))
  print(to_string(length(regex.find_all("", "x"))))
  return 0
}
