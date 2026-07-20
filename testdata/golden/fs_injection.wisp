import "env"
import "fs"
import "process"
fn main() -> int {
  fs.make_dir("--help")
  print(to_string(fs.is_dir("--help")))
  fs.remove_dir("--help")
  print(to_string(fs.is_dir("--help")))

  fs.write_file("-f", "x")
  fs.remove_file("-f")
  print(to_string(fs.file_exists("-f")))

  fs.write_file("-a", "hi")
  fs.rename("-a", "-b")
  print(to_string(fs.file_exists("-b")))
  print(to_string(fs.file_exists("-a")))

  fs.make_dir("$(touch pwned)")
  print(to_string(fs.file_exists("pwned")))
  fs.make_dir("x`touch pwned2`")
  print(to_string(fs.file_exists("pwned2")))

  print(env.get_or("-v BADNAME", "FB"))
  print(env.get_or("$(touch pwned3)", "FB"))
  print(to_string(fs.file_exists("pwned3")))

  fs.which("-v")
  fs.which("$(touch pwned4)")
  print(to_string(fs.file_exists("pwned4")))

  let rc1: int = process.run_status(["printf", "%s", "-rf"])
  print(to_string(rc1))
  let rc2: int = process.run_status(["true", "$(touch pwned5)"])
  print(to_string(rc2))
  print(to_string(fs.file_exists("pwned5")))

  fs.make_dir("injdir")
  fs.write_file("injdir/$(touch pwned6)", "x")
  print(to_string(length(fs.list_dir("injdir"))))
  print(to_string(fs.file_exists("pwned6")))
  return 0
}
