export enum Color: int { Red, Green, Blue }

export fn next(c: Color) -> Color {
    switch (c) {
        case Color.Red   { return Color.Green }
        case Color.Green { return Color.Blue }
        case Color.Blue  { return Color.Red }
    }
}

export fn name(c: Color) -> string {
    switch (c) {
        case Color.Red   { return "red" }
        case Color.Green { return "green" }
        case Color.Blue  { return "blue" }
    }
}
