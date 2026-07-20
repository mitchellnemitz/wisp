fn main() -> int {
  let line: Optional[string] = read_line()
  while (is_some(line)) {
    print("Some(${unwrap(line)})")
    line = read_line()
  }
  print("None")
  return 0
}
