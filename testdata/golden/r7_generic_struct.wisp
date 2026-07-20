struct Box[T] { value: T }

struct Pair[A, B] { first: A, second: B }

fn main() -> int {
    let b: Box[int] = Box { value: 5 }
    print(to_string(b.value))
    b.value = 42
    print(to_string(b.value))
    let s: Box[string] = Box { value: "hello" }
    print(s.value)
    let p: Pair[int, string] = Pair { first: 10, second: "world" }
    print(to_string(p.first))
    print(p.second)
    return 0
}
