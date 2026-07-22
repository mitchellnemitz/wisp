include "./lib/a.wisp" as a

fn main() -> int {
  print(to_string(to_int(a.Code.Ok)))

  let cf: a.Code = a.Code.Fail
  switch (cf) {
    case a.Code.Ok   { print("code-ok") }
    case a.Code.Fail { print("code-fail") }
  }

  print(to_string(a.Code.Ok == a.Code.Fail))

  print(to_string(a.Dir.North))

  let ds: a.Dir = a.Dir.South
  switch (ds) {
    case a.Dir.North { print("dir-north") }
    case a.Dir.South { print("dir-south") }
  }

  print(to_string(a.Dir.North == a.Dir.North))

  print(to_string(to_bool(a.Answer.Yes)))

  let an: a.Answer = a.Answer.No
  switch (an) {
    case a.Answer.No  { print("ans-no") }
    case a.Answer.Yes { print("ans-yes") }
  }

  print(to_string(a.Answer.No == a.Answer.Yes))

  return 0
}
