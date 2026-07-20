struct Box { items: int[] }
fn main() -> int {
  let none_int: Optional[int] = None
  let xs: Optional[int][] = [Some(1), none_int]
  print(debug(xs))
  let nested: Optional[Optional[int]] = Some(Some(1))
  print(debug(nested))
  let b: Box = Box { items: [1, 2] }
  print(debug(b))
  let dm: {string: int[]} = { "k": [1, 2] }
  print(debug(dm))
  return 0
}
