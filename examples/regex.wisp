// Regular-expression tour (POSIX ERE, whole-match only, byte-based under
// LC_ALL=C). Read-only: no filesystem effects, so it is safe to run from
// anywhere (the doctest runs it from the repo root).
import "regex"
import "string"

fn main() -> int {
    print("matches digits: ${to_string(regex.matches("order 42", "[[:digit:]]+"))}")
    print("first match: ${unwrap_or(regex.find("a1b22c", "[0-9]+"), "none")}")
    print("all matches: ${string.join(regex.find_all("a1b22c333", "[0-9]+"), ",")}")
    print("replaced: ${regex.replace("a1b2", "[0-9]+", "#")}")
    return 0
}
