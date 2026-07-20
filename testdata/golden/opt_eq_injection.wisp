fn main() -> int {
  let a: Optional[string] = Some("$(touch pwned)")
  let b: Optional[string] = Some("$(touch pwned)")
  let c: Optional[string] = Some("safe")
  if (a == b) { print("eq") } else { print("neq") }
  if (a == c) { print("eq") } else { print("neq") }
  return 0
}
