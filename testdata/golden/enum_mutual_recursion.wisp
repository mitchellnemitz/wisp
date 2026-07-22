enum Inner { Leaf(int), Wrap(Outer) }
enum Outer { Node(Inner), End }

fn main() -> int {
  let o: Outer = Outer.Node(Inner.Wrap(Outer.End))
  match (o) {
    case Node(i) {
      print("node")
      match (i) {
        case Wrap(inner) {
          print("wrap")
          match (inner) {
            case End { print("end") }
            case Node(_) { print("unreachable") }
          }
        }
        case Leaf(_) { print("unreachable") }
      }
    }
    case End { print("unreachable") }
  }
  return 0
}
