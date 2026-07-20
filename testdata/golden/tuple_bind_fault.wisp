fn boom() -> (int, string) {
    throw error("kaboom")
}

fn main() -> int {
    try {
        let (a: int, b: string) = boom()
        print("unreachable")
    } catch (_) {
        print("caught")
    }
    return 0
}
