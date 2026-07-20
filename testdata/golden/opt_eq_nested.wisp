fn main() -> int {
  let aa: Optional[Optional[int]] = Some(Some(1))
  let ab: Optional[Optional[int]] = Some(Some(1))
  let ac: Optional[Optional[int]] = Some(Some(2))
  let inner_none: Optional[int] = None
  let an: Optional[Optional[int]] = Some(inner_none)
  let nn: Optional[Optional[int]] = None
  if (aa == ab) { print("eq") } else { print("neq") }
  if (aa == ac) { print("eq") } else { print("neq") }
  if (aa == an) { print("eq") } else { print("neq") }
  if (an == an) { print("eq") } else { print("neq") }
  if (nn == nn) { print("eq") } else { print("neq") }
  if (aa == nn) { print("eq") } else { print("neq") }
  return 0
}
