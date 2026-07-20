// program_path() is captured once at top level and is the SAME value whether
// read from main or from a nested function, on every shell (spec P1/AC4). The
// golden harness writes the script as out.sh and runs it by that path, so
// base_name(program_path()) is "out.sh" deterministically.
import "fs"
import "string"
fn nested() -> string {
  return fs.program_path()
}

fn main() -> int {
  let from_main: string = fs.program_path()
  let from_nested: string = nested()
  print(to_string(from_main == from_nested))
  print(fs.base_name(from_main))
  print(to_string(string.is_empty(from_main)))
  return 0
}
