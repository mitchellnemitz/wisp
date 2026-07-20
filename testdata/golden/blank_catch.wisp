fn main() -> int {
    try {
        throw error("boom")
    } catch (_) {
        print("caught")
    }
    return 0
}
