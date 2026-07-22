include "./lib/a.wisp" as a

fn main() -> int {
  let t: a.Tree = a.Tree.Leaf(a.Node { val: 9 })
  match (t) {
    case Leaf(n) { print(to_string(n.val)) }
    case Empty { print("e") }
  }
  return 0
}
