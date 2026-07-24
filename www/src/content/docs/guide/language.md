---
title: Language reference
---


wisp is small, explicit, and C-like. There is no implicit coercion and no
implicit truthiness. `let` bindings and function parameters carry explicit
types; type arguments and a few other positions are inferred (see
[Generics](#generics)). Conditions are
always parenthesized and bodies are always braced. The compiler inserts every
shell quote for you, so the quoting and word-splitting bugs of hand-written
shell do not occur.

## Lexical structure

Source is UTF-8. Identifiers are ASCII: a letter or underscore followed by
letters, digits, or underscores. A non-ASCII identifier is a compile error.
Names beginning with a double underscore are reserved for the compiler.

Comments are line comments only, introduced by `//`. There are no block
comments. A statement ends at a newline or a semicolon; both work, and
formatted output always uses newlines.

```wisp
// a line comment
let count: int = 0   // a trailing comment
```

## Strings

Single-quoted strings are literal. The only escapes are `\'` and `\\`.

Double-quoted strings allow interpolation and the escapes `\n`, `\t`, `\"`,
`\\`, and `\$`. Inside a double-quoted string, `${expr}` evaluates an
expression and inserts its string form. This is the only place a value is
turned into a string implicitly; everywhere else you convert explicitly. To
write a literal dollar-brace, escape the dollar:

```wisp
let x: int = 5
print("real ${x} and literal \${x}")   // real 5 and literal ${x}
```

Any expression may appear inside `${}`:

```wisp
print("sum is ${7 + 3}")
```

String values are inert data. A string that contains shell-active text such as
`$(...)`, backticks, or semicolons is inserted as data and is never executed by
the generated shell.

## Numbers

Integer literals are decimal digits with no sign. A leading `-` is the unary
minus operator, not part of the literal.

Float literals require at least one digit on each side of the dot, such as
`3.14` or `2.0`. A bare `2` is an int. There is no exponent form; `3.` and `.5`
and `1e9` are not valid literals.

```wisp
let a: float = 3.14
let b: float = 2.0
print(to_string(a / b))
print(to_string(-a))
```

## Booleans

The literals are `true` and `false`. bool is its own type; there is no
truthiness, so a condition must already be a bool.

The `bool` conversion is strict and never massages its input. No case folding,
no whitespace trimming, no catch-all.

From an int, `0` is false and any nonzero value, including negatives, is true.

From a float, numeric zero is false and anything else is true.

From a string, only the exact canonical literals convert; anything else aborts:

| input        | result |
|--------------|--------|
| `"true"`     | true   |
| `"false"`    | false  |
| `"True"`     | abort  |
| `" true "`   | abort  |
| `"1"`        | abort  |
| `""`         | abort  |
| anything else| abort  |

To accept looser input, normalize first, for example
`to_bool(string.lower(string.trim(raw)))` or `to_bool(to_int(raw))`.

## Types

The value types are:

- `int`: a signed 64-bit decimal integer, in the range -9223372036854775808 to
  9223372036854775807. A literal outside this range is a compile error in any
  context. Runtime arithmetic whose result exceeds this range is unspecified
  (it wraps two's-complement where the shell supports it; behavior at the exact
  64-bit boundary is shell-dependent), except that the genuinely unrepresentable
  cases -- `math.abs`/negation of the minimum int, and dividing it by `-1` -- are a
  catchable located abort rather than a wrong value.

  Accepted limitation: the exact INT_MIN value `-9223372036854775808` is correct
  in word and print contexts -- `to_string()`, printing, storing in a variable,
  passing as an argument, returning from a function -- on all four supported
  shells (dash, busybox ash, bash, zsh). Arithmetic and comparison at this exact
  boundary have narrower residuals. As an operand of shell arithmetic `$(( ))`,
  wisp emits every variable operand bare (`$(( m + 0 ))`, not `$(( $m + 0 ))`) so
  the shell reads the stored value instead of re-lexing the `2^63` token; an
  INT_MIN value stored in a variable and used in arithmetic is therefore correct
  on dash, bash, and busybox ash (a literal or compile-time constant is spilled to
  the same bare form). The one arithmetic case that still diverges is zsh, whose
  `$(( ))` cannot represent `2^63` at all: an INT_MIN operand there is a loud zsh
  error ("number truncated after 18 digits"), not a silent wrong value. Separately,
  an INT_MIN-valued operand in a `[ ]` numeric comparison can diverge on dash and
  zsh. Programs that need exact cross-shell behavior should avoid `[ ]` comparison
  at the exact INT_MIN boundary and avoid INT_MIN arithmetic on zsh.

- `float`: a finite decimal in the runtime domain: a value whose `%.17g`
  representation is a plain decimal with no exponent character. In practice this
  means magnitudes roughly in the range [1e-4, 1e17). A float literal whose
  `%.17g` form would use exponent notation (magnitude below roughly 1e-4 or at
  or above 1e17), or that overflows to infinity, is a compile error
  (`float literal out of domain`), parallel to the integer range rule. Runtime
  float results that fall outside the domain (produced by arithmetic, not by a
  literal) trigger a located abort.
- `bool`: `true` or `false`.
- `string`: a byte string.
- `error`: a built-in handle with a `message: string` field and a `code: int`
  field (default 0). Construct with `error(msg)` or `error_with(code, msg)`;
  read `e.message` and `e.code`.
- `RunResult`, `Process`: built-in handle types returned by the process builtins
  (`RunResult` from `process.run_full`, `Process` from `process.spawn`). They are valid in `let`,
  parameter, and return annotations; see the [standard library](/guide/stdlib/) for
  their fields and the builtins that produce them.

`void` is a return type only; it is never the type of a value.

The composite types are:

- Arrays, written `T[]`, for example `int[]`, `string[]`, `Point[]`, `int[][]`.
- Dicts, written `{K: V}`, where `K` is `int`, `string`, `bool`, `float`, or a
  value enum, and `V` is any type.
- Structs, named record types you declare.
- Function references, written `fn(T1, T2) -> R`, for example
  `fn(int, int) -> int`.

Structs, arrays, dicts, errors, and function references are reference handles.
Assigning one or passing it to a function shares the same instance. Handles are
opaque: you cannot convert with `to_string()`, compare, or do arithmetic on them.
Use `debug(x)` to render any value structurally for inspection (see the [stdlib guide](/guide/stdlib/#debug)).

### Constant argument domains

When a builtin or the `/` `%` operators receive a compile-time-constant integer
argument whose value cannot satisfy the operation's runtime precondition, the
compiler reports an error at the argument rather than deferring to a runtime
abort. For example, `string.chr(0)`, `string.repeat("x", -1)`, and `5 / 0` are compile
errors. A non-constant argument (a variable or a value computed at runtime) is
still checked by the runtime guard. This mirrors the constant float-literal
domain rule and the integer-literal range rule.

## Variables

Declare a variable with `let`, an explicit type, and an initializer. There is
no uninitialized variable. Local `let` bindings and function parameters always
carry an explicit type; the compiler never infers those. It does infer type
arguments to generic calls, the payload type of `Some`/`Ok`, the for-in binder
type (array element type, or dict key type), and a generic struct's type
arguments from its field values. An empty `[]` or `{}` takes its type from the
surrounding annotation or context, and is an error when there is none.

```wisp
let count: int = 0
count = count + 1
```

Variables are mutable; reassign with `=`. You cannot redeclare a name that is
already visible in the same or an enclosing scope, and you cannot shadow a
parameter. Two separate sibling scopes may reuse a name.

An unused local variable is a warning, not an error. The program still
compiles and the exit code stays 0.

## Constants and immutable bindings

Declare a compile-time constant with `const`, an explicit type, and a constant
expression. A constant expression is a literal, integer arithmetic, boolean
operators, comparisons, string concatenation, or a reference to an earlier
constant.
Constants are folded at compile time and inlined at every use site; no shell
variable is emitted for them.

```wisp
const MAX_RETRIES: int = 3
const TIMEOUT: int = MAX_RETRIES * 2

fn main() -> int {
  print(to_string(TIMEOUT))
  return 0
}
```

Constants may be declared at module scope or inside a function body. A
function-local constant cannot shadow any name already visible in an enclosing
scope; the same strict no-shadowing rule as `let` applies.

Constants are usable wherever a constant expression is required: as `switch`
case values and as function parameter defaults.

A top-level constant may be exported with `export const`, making it referenceable
from another module by qualified name. The reference inlines the producing
module's folded value at the use site, exactly like a same-module constant; no
shell variable crosses the boundary.

```wisp
// lib/util.wisp
export const MAX_RETRIES: int = 3
export const GREETING: string = "hi"
```

Two rules keep `export const` simple. A constant initializer is file-local: it
may not reference another module's constant (`const X: int = util.Y` is a compile
error), so there is no cross-module fold ordering. And exporting is not
transitive: only constants written with `export const` in a module's own source
are exported, so re-exporting an imported constant is a compile error, not a
silent pass-through. `export const _` has no importable name and is a compile
error. `final` is never exportable (it is function-local only).

```wisp
const DEFAULT_PREFIX: string = "info"

fn log(msg: string, prefix: string = DEFAULT_PREFIX) -> string {
  return prefix + ": " + msg
}
```

Float literals are valid in a constant expression, but float arithmetic is a
compile error. Divide or modulo by zero in a constant expression is a compile
error. An integer result that overflows the wisp int range is a compile error.
A declared type that does not match the initializer type is a compile error.

Declare a runtime-immutable local binding with `final`, an explicit type, and
any expression. `final` is function-local only; a `final` at module scope is a
compile error. The initializer is evaluated once at the point of declaration.
Reassignment is a compile error.

```wisp
fn process(items: string[]) -> int {
  final count: int = length(items)
  print(to_string(count))
  return count
}
```

`final` is a reserved keyword and cannot be used as an identifier - a variable
name, parameter name, or function name.

## Blank identifier

`_` is the blank identifier: a write-only sink. You can use it as the binding
target at any binding site to evaluate and discard a value without naming it.

```wisp
let _: int = compute()        // run compute(), keep nothing
_ = compute()                 // same, as a statement
for (_ in items) { tick() }   // iterate, ignore each element
fn log(_: int, msg: string) -> int { print(msg); return 0 }
match (maybe) { case Some(_) { found() } case None { } }
try { risky() } catch (_) { recover() }
```

`_` never enters scope. Multiple `_` bindings in the same scope are legal, and a
blank binding never triggers an unused-variable warning. Because `_` binds
nothing, reading it as a value is a compile error:

```wisp
let x: int = _   // error: cannot use _ as a value
```

In `match`, `_` is the wildcard pattern and a `Variant(_)` arm discards the
payload.

## Operators

The operators, by precedence from highest to lowest:

1. unary `!` and unary `-`
2. `*` `/` `%`
3. `+` `-`
4. `<<` `>>`
5. `&`
6. `^`
7. `|`
8. `<` `<=` `>` `>=`
9. `==` `!=`
10. `&&`
11. `||`

Integer `+ - * / %` truncate division toward zero. Division or modulo by zero
aborts. Float `+ - * /` produce floats; `%` is integer-only, so float modulo is
a compile error. There is no mixing of int and float in one expression; convert
with `to_float(i)` or `to_int(f)` first.

`+` is overloaded: int plus int adds, float plus float adds, and string plus
string concatenates. The operands must share a type.

Comparisons produce a bool. The ordering operators `< <= > >=` and the equality
operators `== !=` are defined for the same set: the primitives int, bool, string,
and float, and enum values (see [Enums](#enums)). Bool orders `false < true`
(false maps to 0, true to 1); an enum orders by its backing value. Equality
additionally accepts comparable `Optional` values (see [Optional](#optional));
ordering does not (Optionals are never ordered). Both operands must share a type.
Comparing values of different types is a compile error.

`&&` and `||` take bools and short-circuit. `!` negates a bool.

### Bitwise operators

The bitwise operators `&` (AND), `|` (OR), `^` (XOR), `<<` (left shift), and
`>>` (right shift) are binary, left-associative, and defined only on `int`. Both
operands must be `int` and the result is `int`; a `float`, `string`, or `bool`
operand is a compile error. There is no unary bitwise complement and no
compound-assignment form. Using `&` or `|` on bool operands is a compile error
that suggests the logical `&&` or `||` instead.

Each lowers to one POSIX arithmetic expansion `$(( l op r ))`, so the result is
identical across dash, busybox ash, bash, and zsh. Negative operands behave as
signed two's-complement: `-1 & 255` is `255`, and `>>` is an arithmetic
(sign-extending) shift, so `-8 >> 1` is `-4`.

All five bitwise operators bind **tighter than comparison**. This is a
deliberate divergence from C, where `&` is looser than `==`. In wisp,

```wisp
flags & MASK == 0
```

parses as `(flags & MASK) == 0`, which is almost always what you want, rather
than C's `flags & (MASK == 0)`. Within the bitwise group, `&` binds tighter than
`^`, which binds tighter than `|`; the shifts `<< >>` bind tighter than all
three; and ordinary arithmetic (`* / %`, `+ -`) binds tighter than the shifts.

A shift amount that is negative or at least the integer width, like an
arithmetic overflow, is platform-defined: the result is whatever the underlying
shell's `$(( ))` produces, exactly as for `*` and `+`. In-range signed values
are portable across the four shells.

A flag-mask example:

```wisp
fn main() -> int {
  let read: int = 4
  let write: int = 2
  let perms: int = read | write
  if ((perms & write) != 0) {
    print("writable")
  } else {
    print("readonly")
  }
  return 0
}
```

Subexpressions and call arguments evaluate strictly left to right.

## Control flow

Every condition is parenthesized and every body is braced. A condition must be
a bool.

### if, else if, else

```wisp
if (x > 10) {
  return "big"
} else if (x > 0) {
  return "small"
} else {
  return "nonpositive"
}
```

### match

`match` dispatches on every variant of an [`Optional`](#optional) or
[`Result`](#result) in one statement. Every variant must be covered; the
compiler rejects a match that leaves any variant unhandled.

```wisp
let r: Result[int] = parse(line)
match (r) {
  case Ok(n) {
    print("parsed: ${to_string(n)}")
  }
  case Err(e) {
    print("error: ${e.message}")
  }
}

let o: Optional[int] = array.find(xs, even)
match (o) {
  case Some(i) { print("found at ${to_string(i)}") }
  case None { print("not found") }
}
```

The scrutinee must be wrapped in parentheses: `match (expr)`. Each arm is
`case Pattern { body }`. Payload bindings (`n`, `e`, `i`) are scoped to their
arm body. To discard the payload, write `Some(_)`.

A payload-carrying variant arm must bind or discard its payload explicitly --
`Some(name)`/`Some(_)` for an `Optional`, and `Ok(name)`/`Ok(_)` or
`Err(name)`/`Err(_)` for a `Result`. A bare payload-carrying variant with no
parentheses (`Some`, `Ok`, `Err`) is a compile error. Payload-free variants
such as `None` are written bare.

```wisp
match (o) {
  case Some(_) { print("present") }   // ok: payload discarded
  case None { print("absent") }       // ok: no payload
}
// `case Some { }` (bare) is a compile error.
```

A trailing `_` arm covers any remaining variants that were not listed
explicitly:

```wisp
match (o) {
  case Some(x) { print(to_string(x)) }
  case _ { }
}
```

A zero-coverage `_` arm (one where all variants are already listed above it)
is allowed but unreachable; the compiler does not count it in return-path
analysis, so functions that return on every explicit arm still type-check.

`match` is exhaustive at compile time: all variants must be covered in one statement.

### while

```wisp
while (i < 3) {
  print("${i}")
  i = i + 1
}
```

### for

The C-style form has an init, a condition, and a post step. The `let` in the
init is scoped to the loop.

```wisp
for (let i: int = 0; i < 3; i = i + 1) {
  print("${i}")
}
```

### for-in

Iterating an array binds each element. Iterating a dict binds each key, in
insertion order.

```wisp
for (x in xs) {
  total = total + x
}

for (k in m) {
  print("${k}=${m[k]}")
}
```

### switch

The subject is an int or a string. Cases use braces, not colons. A case lists
one or more literal values of the subject type. There is no implicit
fall-through, and `default` is required, though it may be empty.

```wisp
switch (code) {
  case 0 {
    return "ok"
  }
  case 1, 2 {
    return "retry"
  }
  default {
    return "fail"
  }
}
```

Case values match literally, even when they contain shell or glob characters.

### break and continue

`break` and `continue` apply to the nearest enclosing `while`, `for`, or
`for-in` loop. They are not used by `switch`. Using `break` outside any loop is
a compile error.

## Functions

A function declares typed parameters and an explicit return type, including
`void`.

```wisp
fn add(a: int, b: int) -> int {
  return a + b
}
```

A non-void function must return on every path. A `throw` also counts as a
terminating path. Recursion is supported.

Trailing parameters may have defaults. A default must be a constant expression,
such as a literal or `stdout` or `stderr`. Omitted trailing arguments are
filled in at the call site.

```wisp
fn label(msg: string, prefix: string = "[info] ") -> string {
  return prefix + msg
}
```

A default on a non-trailing parameter, skipping a middle argument, or a
non-constant default is a compile error.

### main

A program has exactly one `main`, in one of two forms:

```wisp
fn main() -> int { return 0 }
```

```wisp
fn main(args: string[]) -> int {
  for (a in args) {
    print(a)
  }
  return 0
}
```

`args` is an ordinary `string[]`, indexed from 0. `main`'s return value is the
exit code. Missing, duplicate, or wrong-signature `main` is a compile error.

### Function references

A bare function name in a value position becomes a reference; there is no
address-of sigil. The reference's type must match the target's full signature,
and default arguments do not participate, so an indirect call passes every
argument. References can be stored in variables, parameters, return values,
struct fields, array elements, and dict values.

```wisp
fn add(a: int, b: int) -> int { return a + b }
fn main() -> int {
  let f: fn(int, int) -> int = add
  print("indirect=${f(3, 4)}")
  return 0
}
```

#### Referenceable builtins

Builtins can be used as function references, but not all of them the same
way. The compiler groups them into four classes. (Internally the type
checker tracks 9 finer-grained labels in `BuiltinFuncrefClass` — this doc
groups them into the 4 that matter for how you write code.)

**Monomorphic-generatable** (no annotation needed). Any of the 75 builtins in
this class can be referenced directly:

- `string.trim`, `string.lower`, `string.upper`, `string.starts_with`, `string.ends_with`
- `fs.base_name`, `fs.dir_name`, `fs.is_file`, `fs.is_symlink`, `fs.is_dir`, `fs.file_exists`
- `math.sqrt`, `math.exp`, `math.ln`

```wisp
import "array"
import "string"

fn main() -> int {
  let cleaned: string[] = array.map(["  a  ", " b "], string.trim)
  print(cleaned[0])
  print(cleaned[1])
  return 0
}
```

When you write `let f = string.trim` (or pass `string.trim` as a
`fn(string) -> string` argument), the compiler emits a thin wrapper --
`__wisp_builtin_trim` -- that calls the underlying `__wisp_trim` helper using
the standard funcref calling convention. This is called eta-expansion; the
wrapper is tree-shaken and only appears in the output when the builtin is
actually referenced as a value.

**Overloaded** (annotation selects the arm). 7 builtins (`abs`, `min`, `max`,
`clamp`, `sign`, `contains`, `index_of`) have more than one signature; an
explicit funcref type annotation picks which one:

- `math.abs`, `math.min`, `math.max`, `math.clamp`, `math.sign`
- `string.contains`, `string.index_of`

Note: as a value, `string.contains` and `string.index_of` support only the
`string` arm and the `int[]` array arm -- the value form is a subset of the
direct call form, which accepts a wider range of element types.

```wisp
import "math"

fn main() -> int {
  let f: fn(float) -> float = math.abs
  print(to_string(f(-3.5)))
  return 0
}
```

**Generic** (annotation selects the container shape). 12 builtins (`map`,
`filter`, `each`, `reduce`, `sort_by`, `find`, `any`, `all`, `count_where`,
`and_then`, `or_else`, `map_err`) work over more than one container (array,
`Optional`, or `Result`); an explicit funcref type annotation picks which
one:

- `array.map`, `array.filter`, `array.each`, `array.reduce`, `array.sort_by`, `array.find`, `array.any`, `array.all`, `array.count_where`

```wisp
import "array"

fn double(x: int) -> int { return x * 2 }

fn main() -> int {
  let mapper: fn(int[], fn(int) -> int) -> int[] = array.map
  let xs: int[] = [1, 2, 3]
  let ys: int[] = mapper(xs, double)
  print(to_string(ys[0]))
  return 0
}
```

**Rejected.** Some builtins can never be referenced as values, and the
diagnostic you get depends on where the builtin lives:

- A bare (non-module) rejected builtin, like `length` (overloaded on
  string/array but not in the overloaded-funcref table above) or `print`
  (statement-only), reports:

  ```
  builtin "length" cannot be referenced as a function value (it is overloaded or generic); wrap it in a fn
  ```

- A rejected core-module member, like `string.split` (returns an array, not
  a scalar) or `process.run` (opaque argument), reports the SAME
  reason+remedy shape as the bare-builtin case above, naming the module:

  ```
  "split" of module "string" cannot be referenced as a function value (it has no single funcref-shaped scalar lowering); wrap it in a fn
  ```

Wrap either kind in a named function to pass it as a value (wisp has no
inline lambda syntax; the wrapper must be a top-level `fn` declaration):

```wisp
fn length_wrapper(s: string) -> int { return length(s) }

fn main() -> int {
  let f: fn(string) -> int = length_wrapper
  print(to_string(f("hello")))
  return 0
}
```

**Fallible builtins (`math.sqrt`, `math.exp`, `math.ln`) and position
degradation.** When a fallible builtin is called through a funcref and aborts
(for example `math.sqrt` on a negative input), the error location reports the
builtin name (`sqrt`) rather than a source file/line/column. Direct calls to
the same builtin still report a precise source position. This is expected
behavior; the location difference is the only semantic difference between a
builtin funcref and a direct call.

## Generics

A function may declare type parameters in square brackets between its name and
its parameter list, then use them anywhere a type annotation appears: parameter
types, the return type, and `let` annotations in the body. They nest inside the
existing constructors too (`T[]`, `{string: T}`, `Optional[T]`, `fn(T) -> U`).

```wisp
fn identity[T](x: T) -> T {
	return x
}

fn first_of[T](xs: T[]) -> Optional[T] {
	if (length(xs) > 0) {
		return Some(xs[0])
	}
	return None
}

fn main() -> int {
	let xs: int[] = [1, 2, 3]
	print("first: ${unwrap(first_of(xs))}")
	print("id: ${identity(42)}")
	return 0
}
```

The compiler infers the type arguments at each call site by structurally
matching the argument types against the declared parameter types. `first_of([1,
2, 3])` binds `T` to `int`; `first_of(["a"])` binds it to `string`. Unbounded
generics are not monomorphized: every call site uses the same single shell
function. A `comparable`-bounded generic is the same, except when it is
instantiated at `float`: that binding gets its own monomorphized instance (so
`==`/`!=` inside the body compare by numeric identity), while every
non-`float` binding continues to share one erased body.

A type parameter is a name for a type the function does not know, so a value of
that type may only be used in ways that do not depend on what the type is:
bind it to a `let`, pass it as an argument, return it, store it in or read it
out of a collection or `Optional` (`array.push`, index, `length`, `Some`, `unwrap`),
and pass it to a `fn(...)` parameter that mentions it. Operations that need a
concrete type are compile errors on a bare type parameter: arithmetic, the
ordered and equality comparisons, string interpolation, struct field access,
`switch`, and `array.sort`/`array.sum`.

Inference is local and one-pass. A type parameter that no argument constrains
(for example one that appears only in the return type) cannot be inferred; supply
it explicitly (see below).

### Explicit type arguments

Type arguments may be given explicitly at a call site, in square brackets between
the callee and the value arguments: `name[T1, T2](args)`. This works for any
generic function, including qualified cross-module calls (`ns.fn[T](args)`). The
explicit arguments feed the same engine that inference uses, so they must agree
with what the value arguments imply and must satisfy any bound.

Explicit arguments are the only way to call a generic whose type parameter no
argument constrains, such as one that appears only in the return type:

```wisp
import "array"

fn empty_list[T]() -> T[] {
    let xs: T[] = []
    return xs
}

fn main() -> int {
    let xs: int[] = empty_list[int]()
    array.push(xs, 42)
    print("len=${length(xs)}")
    return 0
}
```

A type argument may be any type the language can spell, including composites
(`int[]`, `{string: int}`, `fn(int) -> bool`) and type aliases. Too many or too
few arguments, an argument that contradicts inference, and an argument that
violates a bound are all located compile errors. Only generic callables take type
arguments; supplying them to a non-generic function, a builtin, or a
function-reference call is an error.

The disambiguation from value indexing is purely local: `X[...]` is a type-argument
call only when the bracket contents parse as types and a `(` immediately follows
the `]`. So `a[i]` is indexing and `f[int](x)` is a call. In the rare case of
immediately calling an indexed function value, parenthesize the index:
`(fns[i])(x)`.

### The `comparable` bound

A type parameter may carry the `comparable` bound, written after a colon in the
declaration list. A `comparable` type parameter unlocks `==` and `!=`; the
inferred type must be one of `int`, `bool`, `string`, `float`, or an enum type
(see [Enums](#enums)).

```wisp
fn contains_eq[T: comparable](xs: T[], target: T) -> bool {
	for (x in xs) {
		if (x == target) {
			return true
		}
	}
	return false
}
```

Each parameter is bounded independently (`[T: comparable, U: comparable]`), and a
list may mix bounded and unbounded parameters (`[T: comparable, U]`). `comparable`
is recognized only in this bound position, so an identifier named `comparable`
(a variable, function, or even a type-parameter name) still works elsewhere.
`comparable` and `numeric` (see below) are the only supported bounds; any other
bound word is a compile error.

At each call site the inferred type is checked against the bound. Binding a
`comparable` `T` to a non-comparable type (a struct, array, dict, `Optional`,
`error`, or funcref) is a compile error at the argument that bound it.

Float identity is numeric (`1.0 == 1.00`, `-0.0 == 0.0`), so a `comparable`
generic bound to `float` is monomorphized into its own instance that compares
through the same numeric primitive as the `==` operator, rather than sharing
the byte-text equality of the erased body used for the other comparable
types.

### The `numeric` bound

A type parameter may carry the `numeric` bound to unlock arithmetic and ordered
comparison on a value of that type. A `numeric` `T` accepts `+`, `-`, `*`, `/`,
`<`, `<=`, `>`, `>=`, `==`, `!=`, and unary `-`; `%` is excluded because it is
undefined for float. Only `int` or `float` satisfies the bound at a call site.

```wisp
fn add[T: numeric](a: T, b: T) -> T { return a + b }

fn main() -> int {
    print(to_string(add(3, 4)))
    print(to_string(add(1.5, 2.5)))
    return 0
}
```

`numeric`-bounded generics are monomorphized: the compiler emits one shell
function per concrete type (`__int`, `__float`), so arithmetic dispatches
correctly at runtime.

### Generic structs

A struct may declare type parameters in square brackets after its name. Each
field type may reference the declared parameters; instantiation is written at
the use site.

```wisp
struct Box[T] { value: T }
struct Pair[A, B] { first: A, second: B }

fn main() -> int {
    let b: Box[int] = Box { value: 42 }
    b.value = 99
    print(to_string(b.value))
    let p: Pair[int, string] = Pair { first: 1, second: "one" }
    print(p.second)
    return 0
}
```

The type arguments are written in the `let` annotation. The compiler registers
each concrete instantiation (`Box[int]`, `Box[string]`, ...) on first use and
type-checks field access and assignment against the concrete field types. If the
annotation is omitted, the compiler attempts to infer the type arguments from the
field values; if inference fails it is a compile error.

## Structs

```wisp
struct Point { x: int, y: int }

let p: Point = Point { x: 1, y: 2 }
p.x = 100
print("x is ${p.x}")
```

Fields are typed and comma-separated. Construction names every field. Access
and assignment use `.`. Because structs are handles, assigning one to another
variable shares the instance. Struct fields may themselves be structs, arrays,
or other composites. Constructing a struct with an unknown or missing field is
a compile error.

## Enums

`enum` has two disjoint modes, chosen by whether the declaration carries a
backing annotation:

- **Value enum** (`enum Name: int|string|bool|float { ... }`): a closed set
  of compile-time constants of that backing type. Comparable, usable in
  `switch`, zero runtime cost.
- **Tagged-union enum** (`enum Name { ... }`, no backing, at least one
  variant with a payload): a closed set of constructors, at least one of
  which carries a value. Dispatched only through `match`; it is a handle
  type, like a struct.

A bare enum with no backing and no payload variant at all is a compile
error: it must either declare a backing (to become a value enum) or give at
least one variant a payload (to become a tagged union).

### Value enums

```wisp
enum Color: int { Red, Green, Blue }
enum ExitCode: int { Ok = 0, Fail = 1, Usage = 2 }
enum Priority: int {
    Low,
    Medium,
    High,
}
```

**Backing and defaults.** The backing is `int`, `string`, `bool`, or `float`.
Each backing has its own default rule for a variant with no explicit value:

- `int`: C-style auto-increment. The first variant is `0`; each following
  variant is the previous value plus one. An explicit `= <int literal>`
  (negative allowed) sets the value and reseeds the counter from there.
  ```wisp
  enum E: int { A = 5, B, C }   // A=5, B=6, C=7
  enum Offset: int { Before = -1, Zero = 0, After }  // After=1
  ```
- `string`: the variant's own name. An explicit `= "<string literal>"`
  overrides it.
  ```wisp
  enum Dir: string { North, South = "s" }  // North="North", South="s"
  ```
- `bool`: no default. Every variant must declare an explicit `= true` or
  `= false`.
  ```wisp
  enum Answer: bool { No = false, Yes = true }
  ```
- `float`: no default, like `bool`. Every variant must declare an explicit
  `= <float literal>`.
  ```wisp
  enum Ratio: float { Half = 0.5, Full = 1.0 }
  ```

Every value within one enum must be distinct; a duplicate (including one
produced by int auto-increment or a string default colliding with an
explicit value) is a compile error. For a `float` backing, distinctness is
numeric identity: `0.5` and `0.50` collide, and so do `0.0` and `-0.0`. An
empty enum is a compile error.

**Variant access.** Use `EnumName.VariantName`. For an `int`-backed enum
the access is a constant expression that folds at compile time to its int
value and is usable in `const`/`final` initializers and `switch` cases;
`string`-, `bool`-, and `float`-backed variants are also usable directly as
`switch` subjects and cases.

```wisp
enum Color: int { Red, Green, Blue }
let c: Color = Color.Green
```

**Type semantics.** A value enum is a distinct comparable type. `==` and
`!=` are supported between two values of the same enum. Comparing a value
enum to its backing type, to a different enum, assigning across enum
types, and all arithmetic on enum values (`+`, `-`, `<`, ...) are compile
errors. A value enum type also satisfies the generic
[`comparable` bound](#the-comparable-bound) and the equality-derived
membership builtins (`array.contains`, `array.index_of`, `array.unique`,
`assert_eq`/`assert_ne`, `assert_contains`); this is a wider set than
"supports `==`", since `Optional` of a comparable type also supports `==`
without satisfying `comparable`. A value enum, of any backing, may also be
used as a dict key.

**`to_<backing>()` projection.** `to_int(e)`, `to_string(e)`, `to_bool(e)`,
and `to_float(e)` each return the underlying value, but only the one
matching the enum's own backing: calling any other projection, or calling
the matching one on the wrong backing, is a compile error that names the
correct one to use.

```wisp
enum Color: int { Red, Green, Blue }
print(to_string(to_int(Color.Green)))  // to_string(Color.Green) directly is an error
```

```wisp
enum Dir: string { North, South = "s" }
print(to_string(Dir.North))  // ok: to_string IS the string backing's projection
```

There is no reverse conversion (backing value to enum).

**Exhaustive `switch`.** When the subject of a `switch` is a value enum
(any backing) or a `bool`, the checker requires every variant (or both of
`true`/`false`) to appear in a `case`, unless a `default` arm is present.
Covering all cases removes the need for a `default`. A `default` is still
allowed; omitting a case with no `default` is a compile error. A tagged-union
enum is rejected as a switch subject (match-only).

```wisp
enum Color: int { Red, Green, Blue }
fn describe(c: Color) -> string {
  switch (c) {
    case Color.Red   { return "stop" }
    case Color.Green { return "go" }
    case Color.Blue  { return "sky" }
  }
}
```

Non-enum, non-`bool` (`int`, `string`, or `float`) switches still require a
`default`. A `float` subject matches its case values by numeric identity
(`1.0` matches a case `1.00`; `-0.0` falls through to `default` if there is
no case `0.0`), through the same primitive as `==`, not a byte-text match.

Note: an exhaustive defaultless switch whose every case returns is a
return-path terminator, the same as an exhaustive `match` -- a function
whose non-void body ends with one needs no trailing `return`.

**`debug()` is not defined for a value enum.** Use the `to_<backing>()`
projection, or `switch` over the variants, to render one.

### Tagged-union enums

A bare `enum` (no backing annotation) with at least one payload variant is a
tagged union: each variant is a constructor, some of which carry a typed
payload.

```wisp
enum Expr { IntLit(int), Ident(string), Unit }
```

A payload's type can be `int`, `float`, `string`, `bool`, a struct, an array,
or another enum (including the enclosing enum itself, or a mutual reference
between two enums declared in either order) -- there is no forward-declaration
requirement.

**Construction.** Call a variant like a function: `Enum.Variant(arg)` for a
payload variant, or reference it bare, `Enum.Variant`, for one with none. A
payload variant called with no parens, with wrong arity, or with the wrong
argument type is a compile error; a no-payload variant called with any
arguments (including empty parens `()`) is also a compile error.

```wisp
let e: Expr = Expr.IntLit(3)
let u: Expr = Expr.Unit
```

**`match`.** A tagged-union enum is dispatched with `match`, using bare
(unqualified) constructor names in each case. A payload variant's case must
bind its payload to a name or discard it with `_`; a no-payload variant's
case must be bare (no parens). `match` over a tagged-union enum must be
exhaustive: every variant needs a case, or a wildcard `case _` arm, and a
duplicate case or a case naming an unknown constructor is a compile error.

```wisp
enum Expr { IntLit(int), Ident(string), Unit }

fn describe(e: Expr) -> string {
  match (e) {
    case IntLit(n)   { return to_string(n) }
    case Ident(name) { return name }
    case Unit        { return "u" }
  }
}
```

**Type semantics.** A tagged-union enum is a handle type, like a struct: no
`==`/`!=`, no `to_int()`/`to_string()`/`to_bool()`, not usable as a `switch`
subject, not usable at any of the comparability-gated sites (`array.contains`,
`array.index_of`, `array.unique`, `assert_eq`/`assert_ne`, `assert_contains`,
a `comparable`-bounded generic, or a dict key), and immutable (index-assigning
into one is a compile error). A bare payload variant cannot be used as a
function reference.

**`debug()`.** `debug(e)` renders a tagged-union value as `Variant(payload)`
(or bare `Variant` for a no-payload variant), recursing into struct, enum, and
array payloads and rendering byte-safely (embedded newlines, quotes, and
metacharacters in a string payload come through intact). A value-enum payload
renders by its own backing (a quoted string, or a bare `true`/`false`), not a
raw int. `debug()` on a self-referential or mutually-referential enum value
is rejected at compile time (there is no way to bound the recursion
statically), so `debug()` is only usable on a value whose static type has no
recursive payload cycle reachable from it.

```wisp
enum Expr { IntLit(int), Unit }
print(debug(Expr.IntLit(3)))  // "IntLit(3)"
print(debug(Expr.Unit))       // "Unit"
```

### Both modes

**Module scope.** An enum declared without `export` is module-local, exactly
like a non-exported struct or const. `export enum Name { ... }` (with or
without a backing) makes the enum's variants and the enum itself, as a type,
reachable from an importing module through its namespace alias:

```wisp
// lib/pal.wisp
export enum Color: int { Red, Green, Blue }
export enum Expr { IntLit(int), Unit }
```

```wisp
// app.wisp
include "./lib/pal.wisp" as pal

fn paint() -> int {
    let c: pal.Color = pal.Color.Green
    return to_int(c)
}

fn build() -> pal.Expr {
    return pal.Expr.IntLit(7)
}
```

An exported enum behaves identically to a same-module enum in every respect,
across the module boundary. Construction is qualified (`pal.Expr.IntLit(7)`),
and so is a value enum's `switch` case (`case pal.Color.Green { ... }`), but a
tagged-union `match` case stays bare (`case IntLit(n) { ... }`) even for an
imported enum, since the case names a constructor, not a value. Referencing a
non-exported enum, an unknown variant, or re-exporting an imported enum are
each compile errors.

A generic user enum (`enum Box[T] { ... }`) is not supported; every payload
type must be concrete.

## Type aliases

A `type Name = T` declaration introduces a transparent (structural) alias:
`Name` is a pure synonym for the type annotation `T` and is fully interchangeable
with `T` in every type-annotation position. There is no wrapper, no conversion,
and no distinct identity: a `Miles` value *is* an `int`.

```wisp
type Miles = int
type BinOp = fn(int, int) -> int

fn add(a: int, b: int) -> int {
    return a + b
}

fn apply(f: BinOp, a: int, b: int) -> int {
    return f(a, b)
}

fn main() -> int {
    let distance: Miles = 26
    let doubled: int = distance + distance
    let op: BinOp = add
    print("miles=${distance} doubled=${doubled} sum=${apply(op, 2, 3)}")
    return 0
}
```

The right-hand side is any type annotation: a scalar, an array (`T[]`), a dict
(`{K: V}`), a tuple (`(A, B)`), a funcref (`fn(...) -> R`), an `Optional[T]` or
`Result[T]`, a generic instantiation (`Box[int]`), a struct or enum name, or a
qualified `ns.Type`. An alias name nests like any named type, so `Miles[]` is an
array of the underlying type. A bare `void` right-hand side is rejected (a
funcref return of `void`, as in `fn() -> void`, is fine).

Because an alias resolves to its underlying type before the rest of the compiler
runs, it never appears in type errors, generated shell, or `debug` output; those
show the underlying type. Aliases are module-local (no `export type`), generic
aliases (`type Pair[T] = ...`) are not supported, and an alias cycle
(`type A = B` with `type B = A`) is a compile error.

**Name-as-syntax limitation.** An alias to a struct or enum resolves in type
annotations, but the alias name is not a substitute for the underlying name when
that name is used as *syntax*: to construct a struct write `Point { ... }` (not
the alias), and to name an enum variant write `Color.Red` (not the alias). A
value typed through the alias remains fully interchangeable.

## Arrays

```wisp
let xs: int[] = [3, 4, 5]
xs[1] = 40
array.push(xs, 6)
print("len=${length(xs)} first=${xs[0]}")
```

Arrays are written with square brackets and may be empty or nested. Indexing is
0-based. Reading or writing an out-of-bounds or negative index aborts. `array.push`
appends and grows the array, `length` returns the element count, and `for-in`
iterates in order. `array.reverse(xs)` returns a new reversed array. `string.contains(xs, x)`
tests membership and is defined for arrays of int, bool, string, or an enum
type.

## Dicts

```wisp
let m: {string: int} = { "a": 1, "b": 2 }
m["c"] = 3
print("a=${m["a"]} hasb=${to_string(dict.has(m, "b"))}")
```

Keys are `int`, `string`, `bool`, `float`, or a value enum; values are any
type. A `float` key is identified numerically, the same as `==`: `1.0` and
`1.00` are the same key, and so are `0.0` and `-0.0`. Reading a missing key
aborts, so use `dict.has(d, k)` to test membership first. Assigning a key
inserts it or overwrites it in place. `dict.keys(d)` returns the keys in
insertion order, and `for-in` iterates keys in insertion order. A duplicate
key in a literal is a compile error.

## Multi-line collection literals

Array, dict, and struct literals may span multiple lines. Write each item on
its own line to make the layout multi-line:

```wisp
let ports: int[] = [
    8080,
    8443,
    9090,
]

let config: {string: string} = {
    "host": "localhost",
    "port": "8080",
    "env":  "prod",
}
```

`wisp fmt` preserves the user's layout choice: a literal written on a single
line stays single-line; one written across multiple lines is kept multi-line
(one item per line with a trailing comma, closer on its own line at the
enclosing indent). The formatter never auto-wraps a long single-line literal
and never collapses a multi-line one.

The same applies to multi-line struct declarations:

```wisp
struct Config {
    host: string,
    port: int,
    debug: bool,
}
```

## Trailing commas

A single trailing comma after the final item is accepted in every
comma-separated list in the language:

- array, dict, struct, and tuple literals: `[1, 2, 3,]`, `{"a": 1,}`, `P { x: 1, }`, `(a, b,)`
- struct field declarations: `struct P { x: int, y: int, }`
- function parameter lists: `fn f(a: int, b: int,) -> int { ... }`
- call argument lists: `f(1, 2,)`
- generic type-parameter declarations: `fn id[T,](x: T) -> T { ... }`, `struct Box[T,] { value: T }`
- generic type-argument lists: `Box[int,]`

A trailing comma is optional; the formatter does not add one for single-line
literals. For multi-line literals the formatter always writes a trailing comma
after the last item.

## Optional

`Optional[T]` models a value that may be absent, without a bare `null` and
without a `-1`-style sentinel. It has two states: `Some(x)` and `None`.

```wisp
let a: Optional[int] = Some(42)
let b: Optional[string] = None
let c: Optional[Optional[int]] = Some(Some(1))
```

`Some(x)` has type `Optional[typeof x]`; `x` may be any non-void value,
including a composite or another `Optional`. `Some(None)` is a compile error
(the inner `None` has no expected type); bind the `None` first if you need it:
`let inner: Optional[int] = None` then `Some(inner)`.

`None` denotes absence. It has no type of its own; it takes one from the
expected `Optional[T]` at exactly three sites: a `let` annotation, a `return`,
and an assignment.

```wisp
let x: Optional[int] = None
fn lookup(k: string) -> Optional[int] { return None }
x = None
```

A `None` with no expected `Optional[T]` is a compile error: a bare/standalone
`None`, a call argument (`f(None)`), a collection element (`[None]`), and
`unwrap(None)`/`is_some(None)` are all rejected. The workaround for an argument
is a typed binding: `let n: Optional[int] = None` then `f(n)`.

Read the value only through the access builtins:

| builtin | signature | semantics |
|---|---|---|
| `is_some(o)` | `Optional[T] -> bool` | total; `true` iff `o` is `Some` |
| `is_none(o)` | `Optional[T] -> bool` | total; inverse of `is_some` |
| `unwrap(o)` | `Optional[T] -> T` | the value, or a catchable located abort `unwrap of None` |
| `unwrap_or(o, fallback)` | `Optional[T], T -> T` | total; the value, else `fallback` (evaluated eagerly) |

`Optional` is opaque like `error` and structs: `to_string()`, interpolation, use as
a `switch` subject, and any numeric/index/callee use are compile errors.
`==`/`!=` are supported when the inner type is comparable (`int`, `bool`,
`string`, `float`, an enum type, or a nested comparable `Optional`);
comparison is structural: equal iff both are `None`, or both are `Some` with
equal inner values. A `float` inner value compares by numeric identity, the
same as the `==` operator (`Some(1.0) == Some(1.00)`). `Optional` of a
non-comparable type (arrays, dicts, structs) remains non-comparable, and
`==`/`!=` are still compile errors. For those cases the explicit workaround
applies: `is_none(a) && is_none(b)`, or
`is_some(a) && is_some(b) && unwrap(a) == unwrap(b)`.

The standard-library functions that once used a `-1` sentinel now return
`Optional[int]`: `array.find`, `string.index_of`, `string.last_index_of`. The dict accessor
`dict.get(d, k) -> Optional[V]` joins `dict.has`. See
[the stdlib guide](/guide/stdlib/#optional).

Bind the contained value with `match`, or transform with the combinators
(`array.map`/`and_then`/`array.filter`/`or_else`; see the [stdlib guide](/guide/stdlib/#combinators)).
To render an `Optional` for debugging, use `debug(x)` (see the [stdlib guide](/guide/stdlib/#debug));
`to_string()` is strict and rejects `Optional`.

## Result

`Result[T]` models an operation that produced a value or failed with an error. It
has two states: `Ok(x)` carrying a `T`, and `Err(e)` carrying the built-in
`error` handle. It is the value-form alternative to a thrown/caught abort.

```wisp
let a: Result[int] = Ok(42)
let b: Result[int] = Err(error("boom"))
fn parse(s: string) -> Result[int] {
    if (string.is_empty(s)) { return Err(error("empty input")) }
    return Ok(length(s))
}
```

`Result` takes ONE type parameter: the success type. The error payload is always
the built-in `error` handle, so `Result[T]` is conceptually `Result[T, error]`
(written `Result[T]`). `Ok(x)` has type `Result[typeof x]`, inferred fully like
`Some(x)`. `Err(e)` requires `e: error`; like `None`, it has no success type of
its own and takes one from the expected `Result[T]` at exactly three sites: a
`let` annotation, a `return`, and an assignment. An `Err` with no expected
`Result[T]` (a bare `Err`, a call argument, a collection element) is a compile
error, and `Ok(None)`/`Ok(Err(...))` is rejected (the inner literal has no
context).

Read the contents only through the access builtins:

| builtin | signature | semantics |
|---|---|---|
| `is_ok(r)` | `Result[T] -> bool` | total; `true` iff `r` is `Ok` |
| `is_err(r)` | `Result[T] -> bool` | total; inverse of `is_ok` |
| `unwrap(r)` | `Result[T] -> T` | the value, or a catchable located abort carrying the `Err`'s `.message` |
| `unwrap_err(r)` | `Result[T] -> error` | the error, or a catchable located abort `unwrap_err of Ok` |
| `unwrap_or(r, fallback)` | `Result[T], T -> T` | total; the value, else `fallback` (evaluated eagerly) |

`unwrap` and `unwrap_or` are overloaded: they accept either an `Optional[T]` or a
`Result[T]`, dispatched on the static type of the argument. `is_ok`/`is_err`/
`unwrap_err` are `Result`-only; `is_some`/`is_none` are `Optional`-only.

`Result` is opaque, like `Optional`/`error`/structs: `==`/`!=`, `to_string()`,
interpolation, and use as a `switch` subject are compile errors. `Result` is
non-comparable for every `T` because its `Err` arm carries the opaque `error`
handle; use `is_ok`/`unwrap` or `match` instead. To render a `Result` for
debugging, use `debug(x)` (see the [stdlib guide](/guide/stdlib/#debug)); `to_string()`
is strict and rejects `Result`. Bind a `Result` with `match`, or transform with
the combinators (`array.map`/`and_then`/`or_else`/`map_err`; see the [stdlib guide](/guide/stdlib/#combinators)).

## Tuples

A tuple is a fixed-arity, immutable, ordered collection of values of heterogeneous types.

```wisp
let t: (int, string) = (1, "hello")
```

Tuple types require at least two elements: `(T1, T2)`, `(T1, T2, T3)`, etc. Single-element and empty-tuple forms are parse errors.

### Element access

Access elements with a constant integer literal index (0-based).

```wisp
let x: int = t[0]
let y: string = t[1]
```

The index must be a compile-time integer literal. A variable or arithmetic expression as an index is a compile error. An out-of-range index is also a compile error.

### Tuples as function types

Tuples work as parameter and return types.

```wisp
fn pair(a: int, b: string) -> (int, string) {
    return (a, b)
}
```

### Immutability and opacity

Tuples are immutable: `t[0] = x` is a compile error. Tuples are opaque: `==`/`!=`, `to_string()`, and `switch` on a tuple are compile errors. To render a tuple for debugging, use `debug(t)` (see the [stdlib guide](/guide/stdlib/#debug)).

### Nested tuples

Nested tuple access chains naturally: `t[0][1]` reads element 1 of the inner tuple held at element 0.

### Tuple destructuring

Instead of binding a tuple to one name and indexing it, you can bind its
elements directly in a single `let` or `final` statement:

```wisp
fn pair() -> (int, string) {
    return (42, "hello")
}

fn main() -> int {
    let (a: int, b: string) = pair()
    print("${to_string(a)} ${b}")
    return 0
}
```

The pattern is wrapped in parens -- mirroring the tuple type `(T1, T2)` and the
fn-parameter list `fn f(a: int, b: string)`, not square brackets, which belong
to arrays, indexing, and generics. Each binding slot carries a mandatory type
annotation, exactly like every other `let`/`final` binding. The pattern requires
at least two slots (k >= 2).

`final` destructuring binds immutable names; `let` binds mutable ones.

```wisp
fn minmax(xs: int[]) -> (int, int) {
    return (xs[0], xs[0])
}

fn main() -> int {
    final (lo: int, hi: int) = minmax([3, 1, 4])
    print("lo=${to_string(lo)} hi=${to_string(hi)}")
    return 0
}
```

To discard an element, use the bare blank identifier `_`. A bare `_` imposes no
type constraint on that element. An annotated discard `_: Type` is also accepted
and checks the element type without binding a name.

```wisp
fn pair() -> (int, string) {
    return (0, "kept")
}

fn main() -> int {
    let (_, msg: string) = pair()
    print(msg)
    return 0
}
```

The RHS is evaluated exactly once regardless of how many slots the pattern has.
A trailing comma in the pattern is accepted: `let (a: int, b: string,) = ...`.

## Error handling

An `error` is a handle carrying a `message: string` and a `code: int` (default
0). Construct one with `error(msg)`, or with `error_with(code, msg)` to set the
code; read the fields with `e.message` and `e.code`.

`throw` raises an error. The thrown expression must have type `error`. An
uncaught throw aborts, prints the message, and exits 1.

A `try` block has a required `catch` and an optional `finally`. The caught
error is bound in the handler and scoped to it.

```wisp
try {
  print(to_string(a / b))
} catch (e) {
  print(e.message)
}
```

Inside a `try`, including any function it calls, a runtime fault such as
division by zero, a bad conversion, an out-of-bounds index, or a missing dict
key becomes a catchable error instead of aborting. So does a `throw`. On the
first fault the rest of the body is skipped and the handler runs. Mutations made
before the fault persist after the `try`, because the body runs in the current
shell rather than a subshell.

`finally` always runs, exactly once, on every path: clean, caught, or
rethrown. Throwing inside a `catch` propagates to the next enclosing `try`, but
the current `finally` runs first.

```wisp
try {
  try {
    throw error("inner")
  } catch (e) {
    throw error("from-inner")
  } finally {
    print("inner-finally")
  }
} catch (e) {
  print("outer-catch: ${e.message}")
} finally {
  print("outer-finally")
}
```

A thrown or faulted message keeps shell-active text inert; it is printed as
data, never executed.

In this version, `return` and a syntactic `exit` call are not allowed inside a
`try`, `catch`, or `finally` body. Structure the code so the `try` is the last
thing before the exit you want, then return after it. `break` and `continue` are
allowed only when they target a loop that is itself fully inside the try body; a
`break` or `continue` that would escape the enclosing `try` (targeting a loop
opened outside it) is a compile error.

## Modules

A program can span several files and pull in libraries. Modules are a
compile-time concept: the compiler links every used symbol from every reachable
module into the one output script, tree-shaken to what `main` actually reaches.
There is no module system in the generated shell.

A top-level function or struct marked `export` is visible to other modules;
anything not exported is private to its file.

```wisp
export struct Point { x: int, y: int }

export fn manhattan(p: Point) -> int {
    return math.abs(p.x) + math.abs(p.y)
}

fn helper() -> int {
    return 1
}
```

Bring another module into scope under a namespace, then use its symbols
qualified. A local file is pulled in with `include`, resolved relative to the
including file:

```wisp
include "./lib/geo.wisp" as geo

fn area(w: int, h: int) -> int {
    let origin: geo.Point = geo.Point { x: 0, y: 0 }
    return geo.manhattan(origin) + w * h
}
```

An installed package is pulled in with `import "owner/repo"`. The string is a
logical name, never a URL. With no alias the namespace is the package's declared
name; `as` overrides it:

```wisp
import "acme/strutil" as s

fn loud(name: string) -> string {
    return s.shout(name)
}
```

An exported constant is used through the same qualified form, in value position:

```wisp
import "acme/util" as util

fn retries() -> int {
    return util.MAX_RETRIES
}
```

Rules:

- Access is always qualified: `geo.Point`, `geo.manhattan(p)`, `s.shout(name)`.
  There is no unqualified bring-into-scope form.
- Only the file passed to `wisp build`/`run` may define `fn main`. An imported or
  included module that defines `main` is a compile error.
- Visibility is not transitive: importing a module does not expose that module's
  own imports.
- `include` resolves a relative `.wisp` path that stays inside your project, and
  cannot reach into the installed-package directory; packages are reached only
  through `import`.
- The installed packages live under a `.wisp/` directory at the project root,
  placed there by the separate package tool. The compiler only reads what is
  already present; it does not fetch anything.

The default namespace for an `include` is the file's base name without `.wisp`
(`util.wisp` becomes `util`); if that is not a valid identifier, an `as alias` is
required. A `wisp.json` at a directory marks it as the project root and names a
package's entry file; its `name` is the package's default import namespace.

## Testing

Test files are named `*_test.wisp`. A test file has no `fn main`; the compiler generates
the entry point when it detects the `_test.wisp` suffix.

### The test construct

```
test ("<name>") { <body> }
```

`"<name>"` is a string literal. Duplicate names within a file are a compile error. The
body is a sequence of statements. A test exits by completing normally (pass), by an
assertion failure (fail), or by calling `skip(reason)` (skip).

```wisp
test ("addition is commutative") {
    assert_eq(1 + 2, 2 + 1)
}

test ("placeholder") {
    skip("not written yet")
}
```

The `test` construct is only valid in `*_test.wisp` files; using it elsewhere is a
compile error.

Lifecycle functions `setup() -> void` and `teardown() -> void`, when defined at the top
level of a test file, run before and after each test body, respectively. `teardown` runs
even if the test failed or was skipped.

See the [testing guide](/guide/testing/) for assertions, `skip`, `test_tmpdir`, and
`wisp test` usage.

### Paren-keyword principle

Every construct keyword in wisp takes a parenthesized head. `test ("name") { }` follows
the same pattern as `fn`, `if (...)`, `while (...)`, `for (...)`, and `match (...)`.
There is never a non-parenthesized function-like keyword in wisp, under any
circumstances. This applies to built-in constructs and is the invariant that defines the
shape of wisp syntax.

## Reserved names

Builtin names and the constants `stdout` and `stderr` cannot be redeclared or
shadowed. `Optional`, `Result`, `RunResult`, and `Process` are reserved type
names, and `Some`/`None`/`Ok`/`Err` are reserved constructor/literal names; none
may be used for a function, variable, parameter, or struct. Names beginning with
a double underscore are reserved for the
compiler. `export`, `import`, and `include` are the module keywords (see
[Modules](#modules)). Every reserved keyword has a meaning; there are no
inert "reserved for future use" keywords. The canonical list of identifiers a
user may not define is `internal/types/accessors.go` `ReservedNames()`.

See the [standard library](/guide/stdlib/) for the full list of builtins.
