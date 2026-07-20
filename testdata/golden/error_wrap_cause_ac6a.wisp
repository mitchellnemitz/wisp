fn main() -> int {
    throw wrap(error("inner"), "top")
}
