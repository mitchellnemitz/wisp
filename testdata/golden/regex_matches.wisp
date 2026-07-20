import "regex"
fn main() -> int {
  print(to_string(regex.matches("hello42", "[0-9]+")))
  print(to_string(regex.matches("hello", "[0-9]+")))
  print(to_string(regex.matches("hat", "^h")))
  print(to_string(regex.matches("hat", "o$")))
  print(to_string(regex.matches("cat", "c|d")))
  print(to_string(regex.matches("xyz", "c|d")))
  print(to_string(regex.matches("a1", "[[:digit:]]")))
  print(to_string(regex.matches("aaa", "a{2,3}")))
  print(to_string(regex.matches("a", "a{2,3}")))
  return 0
}
