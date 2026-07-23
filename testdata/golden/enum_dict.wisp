import "dict"
enum Code: int { A = 1, B = 2 }
enum E: string { A = "x", B = "y" }
enum Flag: bool { On = true, Off = false }

fn main() -> int {
  let ci: {Code: string} = {}
  ci[Code.A] = "one"
  ci[Code.B] = "two"
  print("${dict.get_or(ci, Code.A, "?")}")
  let se: {E: int} = {}
  se[E.A] = 1
  se[E.B] = 2
  print("${dict.get_or(se, E.A, -1)}")
  print("${length(dict.keys(se))}")
  let fl: {Flag: int} = {}
  fl[Flag.On] = 100
  fl[Flag.Off] = 200
  print("${dict.get_or(fl, Flag.On, -1)}")
  print("${length(dict.keys(fl))}")
  return 0
}
