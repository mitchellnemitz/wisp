type Miles = int

struct Trip { start: Miles, distance: Miles }

fn total(t: Trip) -> Miles {
    return t.start + t.distance
}

fn main() -> int {
    let t: Trip = Trip { start: 10, distance: 32 }
    let d: Miles = total(t)
    let n: int = d
    print("total=${d}")
    print("asint=${n}")
    return 0
}
