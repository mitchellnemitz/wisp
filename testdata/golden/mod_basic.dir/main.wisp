include "./lib/geo.wisp" as geo
import "acme/strs" as s

fn main() -> int {
    let p: geo.Point = geo.Point { x: 3, y: 4 }
    let label: string = s.shout("pt")
    let dist: int = geo.manhattan(p)
    print("${label}=${to_string(dist)}")
    return 0
}
