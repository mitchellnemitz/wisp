import "fs"
import "process"
fn main() -> int {
  // The hostile set: command substitution, backticks, semicolon, glob star,
  // spaces, and a NON-trailing newline. The newline is embedded (not at the
  // end), so command-substitution trailing-newline stripping does not affect it.
  let hostile: string = "$(touch pwned); `touch pwned2`; a * b\n_end"

  // (b) env VALUE round-trip: the var is set via the augmented environment;
  // the child reads it with printf %s (no newline added), and run_env captures
  // via command substitution. The hostile bytes come back verbatim.
  let e: {string: string} = {"V": hostile}
  let val_got: string = process.run_env(["sh", "-c", "printf %s \"$V\""], e)
  print(to_string(val_got == hostile))

  // (c) argv ELEMENT round-trip: the hostile string is passed as a single argv
  // element to printf %s. No shell interprets it; it reaches printf as one
  // argument and is printed verbatim. This proves the element is not mangled,
  // split, or re-evaluated.
  let argv_e: {string: string} = {"X": "1"}
  let arg_got: string = process.run_env(["printf", "%s", hostile], argv_e)
  print(to_string(arg_got == hostile))

  // (a) no side-effect file was created by any of the above.
  print(to_string(fs.file_exists("pwned")))
  print(to_string(fs.file_exists("pwned2")))
  return 0
}
