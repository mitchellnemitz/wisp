// Demonstrates parse_args: valued flags, boolean switches, and positionals.
// Parses a fixed argument list so the output is deterministic under the doctest harness.
import "dict"
import "string"

fn main() -> int {
    let fixed: string[] = ["--name", "ada", "--verbose", "report.txt", "notes.txt"]
    // value_flags lists the flag names that consume the next token as a value.
    let (vals: {string: string}, sw: string[], files: string[]) = parse_args(fixed, ["--name"])
    // Read a valued flag; falls back to "world" when absent.
    let name: string = unwrap_or(dict.get(vals, "--name"), "world")
    print("hello, ${name}")
    // Check a boolean switch by membership.
    let verbose: bool = string.contains(sw, "--verbose")
    print("verbose: ${to_string(verbose)}")
    // Positionals come through unchanged.
    print("files: ${string.join(files, ", ")}")
    // The `=` form produces the same result.
    let alt: string[] = ["--name=ada", "--verbose", "report.txt", "notes.txt"]
    let (vals2: {string: string}, _, _) = parse_args(alt, ["--name"])
    print("= form name: ${unwrap_or(dict.get(vals2, "--name"), "?")}")
    // Absent flag: get returns None, so unwrap_or yields the fallback.
    let out: string = unwrap_or(dict.get(vals, "--out"), "stdout")
    print("out: ${out}")
    return 0
}
