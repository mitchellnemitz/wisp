include "./lib/pal.wisp" as pal

struct Badge { c: pal.Color }

// echo exercises FR-004a (parameter) and FR-004b (return) at the CONSUMING
// module's own function boundary, annotated with the namespace-qualified
// `pal.Color` -- distinct from lib's `next`/`name`, which annotate the enum by
// its bare local name inside the defining module. Passing a variant in and
// getting it back proves SC-002's round-trip across a module-defined boundary.
fn echo(c: pal.Color) -> pal.Color {
    return c
}

fn main() -> int {
    let a: pal.Color = pal.Color.Green
    let b: pal.Color = pal.next(a)

    if (a == pal.Color.Green) {
        print("a-is-green")
    }
    if (a != b) {
        print("a-ne-b")
    }

    print(to_string(to_int(a)))
    print(to_string(to_int(b)))

    print(pal.name(a))
    print(pal.name(b))

    let badge: Badge = Badge { c: b }
    print(pal.name(badge.c))

    let e: pal.Color = echo(pal.Color.Blue)
    if (e == pal.Color.Blue) {
        print("e-is-blue")
    }
    print(to_string(to_int(e)))
    print(pal.name(e))

    switch (b) {
        case pal.Color.Red   { print("r") }
        case pal.Color.Green { print("g") }
        case pal.Color.Blue  { print("bl") }
    }

    return 0
}
