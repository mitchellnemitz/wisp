fn main() -> int {
  let a: Optional[string] = Some("hello")
  let b: Optional[string] = Some("hello")
  let c: Optional[string] = Some("world")
  let n: Optional[string] = None
  if (a == b) { print("eq") } else { print("neq") }
  if (a == c) { print("eq") } else { print("neq") }
  if (n == n) { print("eq") } else { print("neq") }
  if (a != n) { print("neq") } else { print("eq") }
  return 0
}
