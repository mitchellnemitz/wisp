fn main() -> int {
  let line: Optional[string] = read_line()
  if (is_some(line)) {
    print("Some(${unwrap(line)})")
  } else {
    print("None")
  }
  return 0
}
