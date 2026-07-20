import "string"
fn main() -> int {
    // reverse_string: result, source unchanged, empty, single
    let s: string = "hello"
    let r: string = string.reverse(s)
    print(r)
    print(s)
    print(string.reverse(""))
    print(string.reverse("a"))

    // ord: ASCII codepoints and high-byte round-trips
    print(to_string(string.ord("A")))
    print(to_string(string.ord("a")))
    print(to_string(string.ord("hello")))
    print(to_string(string.ord(string.chr(200))))
    print(to_string(string.ord(string.chr(255))))

    // chr: ASCII round-trips; extremes have length 1
    print(string.chr(65))
    print(string.chr(97))
    print(to_string(length(string.chr(1))))
    print(to_string(length(string.chr(255))))

    // chr(ord(char_at(s, i))) round-trips
    let t: string = "hello"
    print(string.chr(string.ord(string.char_at(t, 0))))
    print(string.chr(string.ord(string.char_at(t, 4))))

    // ord on empty string: catchable
    try {
        print("ord-ok:" + to_string(string.ord("")))
    } catch (e) {
        print("ord-caught")
    }

    // chr out of range: catchable for 0, -1, 256 (codes via vars so the
    // out-of-domain value is a runtime fault, not a compile-time rejection)
    let c0: int = 0
    try {
        print("chr0-ok:" + string.chr(c0))
    } catch (e) {
        print("chr0-caught")
    }
    let cn1: int = -1
    try {
        print("chrn1-ok:" + string.chr(cn1))
    } catch (e) {
        print("chrn1-caught")
    }
    let c256: int = 256
    try {
        print("chr256-ok:" + string.chr(c256))
    } catch (e) {
        print("chr256-caught")
    }

    // byte-reversal of non-ASCII bytes (0xC3=195, 0xA9=169)
    let two_byte: string = string.chr(195) + string.chr(169)
    let rev: string = string.reverse(two_byte)
    print(to_string(string.ord(string.char_at(rev, 0))))
    print(to_string(string.ord(string.char_at(rev, 1))))

    return 0
}
