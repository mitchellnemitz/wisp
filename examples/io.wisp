// A tour of I/O builtins, kept side-effect-free so it is deterministic: it
// runs a command and reads the environment, but writes no files.
import "env"
import "process"

fn main() -> int {
    // run a command and capture its standard output (trailing newline stripped).
    let greeting: string = process.run(["echo", "hello from run"])
    print("ran: ${greeting}")
    // join two captured outputs.
    let a: string = process.run(["echo", "first"])
    let b: string = process.run(["echo", "second"])
    print("both: ${a} / ${b}")
    // PATH is always set, so has_env is a deterministic true here.
    print("has PATH: ${to_string(env.has("PATH"))}")
    // env reads it; the value varies, so just report its presence by length sign.
    let path: string = env.get("PATH")
    let nonEmpty: bool = length(path) > 0
    print("PATH is non-empty: ${to_string(nonEmpty)}")
    return 0
}
