const RETVAL: int = 42

fn fetch(x: int = RETVAL) -> int {
  return x
}

fn main() -> int {
  print(to_string(fetch()))
  print(to_string(fetch(7)))
  return 0
}
