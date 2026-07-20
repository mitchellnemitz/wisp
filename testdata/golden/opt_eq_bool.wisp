fn main() -> int {
  let a: Optional[bool] = Some(true)
  let b: Optional[bool] = Some(true)
  let c: Optional[bool] = Some(false)
  let n: Optional[bool] = None
  if (a == b) { print("eq") } else { print("neq") }
  if (a == c) { print("eq") } else { print("neq") }
  if (n == n) { print("eq") } else { print("neq") }
  if (a != n) { print("neq") } else { print("eq") }
  return 0
}
