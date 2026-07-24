// Collections-core standard library: array and dict helpers.
import "array"
import "dict"
import "string"

fn show(x: int) -> string {
    return to_string(x)
}

fn even(x: int) -> bool {
    return x % 2 == 0
}

fn desc(a: int, b: int) -> bool {
    return a > b
}

fn main() -> int {
    let xs: int[] = [3, 1, 4, 1, 5, 9, 2, 6]
    print(string.join(array.map(array.sort(xs), show), " "))
    print(string.join(array.map(array.sort_by(xs, desc), show), " "))
    print(to_string(unwrap_or(array.find(xs, even), -1)))
    print(to_string(array.any(xs, even)) + " " + to_string(array.all(xs, even)))
    print(string.join(array.map(array.slice(xs, 2, 5), show), " "))
    print(string.join(array.map(array.concat([0], xs), show), " "))
    print(to_string(array.sum(xs)))
    print(string.join(array.map(array.range(5), show), " "))
    print(to_string(array.first(xs)) + " " + to_string(array.last(xs)))
    let inv: {string: int} = { "apple": 3, "pear": 2, "plum": 5 }
    print(string.join(dict.keys(inv), ","))
    print(string.join(array.map(dict.values(inv), show), ","))
    print(to_string(unwrap_or(dict.get(inv, "pear"), 0)) + " " + to_string(unwrap_or(dict.get(inv, "fig"), 0)))
    dict.remove(inv, "pear")
    print(string.join(dict.keys(inv), ","))
    let more: {string: int} = dict.merge(inv, { "apple": 10, "kiwi": 7 })
    print(string.join(dict.keys(more), ",") + " apple=" + to_string(more["apple"]))
    return 0
}
