fn main() -> int {
  let a: Optional[int[]] = Some([1, 2])
  let b: Optional[int[]] = Some([3, 4])
  let r: bool = a == b
  return 0
}
