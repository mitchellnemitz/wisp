// Structs: declaration, construction, field access and mutation.
import "math"

struct Point { x: int, y: int }

fn manhattan(p: Point) -> int {
    return math.abs(p.x) + math.abs(p.y)
}

fn main() -> int {
    let p: Point = Point { x: 3, y: 0 - 4 }
    print("point (${p.x}, ${p.y})")
    p.x = 10
    print("moved to (${p.x}, ${p.y})")
    print("manhattan distance: ${manhattan(p)}")
    return 0
}
