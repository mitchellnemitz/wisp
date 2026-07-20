import "fs"
import "regex"
fn main() -> int {
  // Hostile pattern / subject / replacement bytes are inert DATA (ENVIRON, not
  // -v; constant awk program), so no shell or awk code is ever executed.
  print(to_string(regex.matches("hello", "; rm -rf /")))
  let a: Optional[string] = regex.find("x", "$(touch pwned1)")
  print(to_string(is_some(a)))
  let b: string[] = regex.find_all("y", "`touch pwned2`")
  print(to_string(length(b)))
  print(regex.replace("z", "no", "$(touch pwned3)"))
  print(to_string(regex.matches("$(touch pwned4)", "rm")))

  // A replacement containing $(...) / backticks is inserted literally.
  print(regex.replace("Q", "Q", "$(touch pwned5)"))
  print(regex.replace("Q", "Q", "`touch pwned6`"))

  // \. matches a LITERAL dot and not any char: proof ENVIRON was used, not -v
  // (under -v the \. would be C-unescaped to . and "axb" would match).
  print(to_string(regex.matches("a.b", "a\\.b")))
  print(to_string(regex.matches("axb", "a\\.b")))

  // No pwned* file was ever created.
  print(to_string(fs.file_exists("pwned1")))
  print(to_string(fs.file_exists("pwned2")))
  print(to_string(fs.file_exists("pwned3")))
  print(to_string(fs.file_exists("pwned4")))
  print(to_string(fs.file_exists("pwned5")))
  print(to_string(fs.file_exists("pwned6")))
  return 0
}
