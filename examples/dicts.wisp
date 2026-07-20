// Dicts: literals, insert, membership, iteration in insertion order, keys.
import "dict"

fn main() -> int {
    let ages: {string: int} = { "alice": 30, "bob": 25 }
    ages["carol"] = 41
    for (name in ages) {
        print("${name} is ${ages[name]}")
    }
    print("has dave: ${to_string(dict.has(ages, "dave"))}")
    print("number of people: ${length(dict.keys(ages))}")
    return 0
}
