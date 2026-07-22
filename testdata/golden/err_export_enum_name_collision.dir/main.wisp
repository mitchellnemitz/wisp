// Deliberately pairs an EXPORTED enum with a NON-exported struct of the same
// name: FR-012 says the collision applies "regardless of export status," so
// mixing the two export states proves the check does not depend on both being
// exported (or on export status at all).
struct Color { r: int }
export enum Color: int { Red, Green }

fn main() -> int {
    return 0
}
