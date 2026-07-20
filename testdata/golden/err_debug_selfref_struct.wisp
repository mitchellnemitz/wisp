struct Node { next: Optional[Node] }
fn main() -> int {
  let empty: Optional[Node] = None
  let n: Node = Node { next: empty }
  let _ : string = debug(n)
  return 0
}
