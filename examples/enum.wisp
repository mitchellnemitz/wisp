// Enums: declaration, variant access, exhaustive switch, to_int() conversion.
enum Color { Red, Green, Blue }

enum ExitCode { Ok = 0, Fail = 1, Usage = 2 }

fn describe(c: Color) -> string {
    switch (c) {
        case Color.Red {
            return "stop"
        }
        case Color.Green {
            return "go"
        }
        case Color.Blue {
            return "sky"
        }
    }
    return ""
}

fn main() -> int {
    let c: Color = Color.Green
    print("color: ${describe(c)}")
    print("value: ${to_int(c)}")
    print("is green: ${to_string(c == Color.Green)}")
    print("is red: ${to_string(c == Color.Red)}")
    let code: ExitCode = ExitCode.Usage
    print("exit code: ${to_int(code)}")
    return 0
}
