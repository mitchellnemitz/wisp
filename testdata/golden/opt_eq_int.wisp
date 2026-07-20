fn main() -> int {
  let a: Optional[int] = Some(1)
  let b: Optional[int] = Some(1)
  let c: Optional[int] = Some(2)
  let n: Optional[int] = None
  if (a == b) { print("eq") } else { print("neq") }
  if (a == c) { print("eq") } else { print("neq") }
  if (a == n) { print("eq") } else { print("neq") }
  if (n == n) { print("eq") } else { print("neq") }
  if (a != b) { print("neq") } else { print("eq") }
  if (a != c) { print("neq") } else { print("eq") }
  return 0
}
