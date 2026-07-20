import "math"
export struct Point { x: int, y: int }

export fn manhattan(p: Point) -> int {
    return math.abs(p.x) + math.abs(p.y)
}
