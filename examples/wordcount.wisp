// A small program that uses command-line arguments: count how many times each
// argument word appears. Run it with, for example:
//   wisp run examples/wordcount.wisp apple banana apple
import "dict"

fn main(args: string[]) -> int {
    let counts: {string: int} = {}
    for (w in args) {
        if (dict.has(counts, w)) {
            counts[w] = counts[w] + 1
        } else {
            counts[w] = 1
        }
    }
    for (w in counts) {
        print("${w}: ${counts[w]}")
    }
    return 0
}
