" Vim syntax file for the wisp language.
" Highlighting groups mirror the VSCode TextMate grammar; the keyword/type/
" builtin/const word lists are kept in sync with the compiler by an automated
" drift-guard test (internal/editors).

if exists("b:current_syntax")
  finish
endif

" Identifier word sets. Each scope lives in a single-line `syn keyword` cluster
" with a fixed group name (wispKeyword/wispType/wispBuiltin/wispConstant), which
" the drift-guard test parses and compares against the compiler's sets. wispType
" is the one exception: `string` overlaps with the `string.member` namespace
" qualifier, so it is carved out of the keyword cluster into its own `syn match`
" below (the drift-guard's singleBoundaryWord extractor folds that form back in).
syn keyword wispKeyword break case catch const continue default else enum export false final finally fn for if import include let match return struct switch test throw true try type while
syn keyword wispType Optional Process RunResult bool error float int void
syn match   wispType "\<string\>\.\@!"
syn keyword wispBuiltin and_then assert assert_contains assert_eq assert_err assert_ne assert_none assert_ok assert_some cause debug error_with exit is_err is_none is_ok is_some length map_err now on_exit on_signal or_else parse_args print read_line read_secret read_stdin set_stdin sleep test_tmpdir to_bool to_float to_int to_string unwrap unwrap_err unwrap_or wrap
" `skip` is a reserved :syn keyword argument, so it is matched separately
" (still group wispBuiltin; the drift-guard test folds single-word `syn match`
" contributions into the group's set). `contains` was moved to a module
" (string.contains/array.contains) by the flat-surface removal, so it is no
" longer a bare builtin and is deliberately absent here.
syn match wispBuiltin "\<skip\>"
syn keyword wispConstant Err None Ok Some stderr stdout

" Namespaced core-module calls: the qualifier (e.g. `string` in `string.trim`)
" is a namespace, not a type, and the member is a builtin (reusing wispBuiltin,
" no new member group). \ze/lookbehind keep the two matches non-overlapping and
" require a lowercase member, so a type-member like `json.Value` is unaffected.
syn match wispNamespace "\<\%(array\|dict\|env\|fs\|json\|math\|process\|regex\|string\)\>\ze\.[a-z]"
syn match wispBuiltin "\%(\<\%(array\|dict\|env\|fs\|json\|math\|process\|regex\|string\)\.\)\@<=[a-z][a-z0-9_]*"

" Comments.
syn match wispComment "//.*$" contains=@Spell

" Numbers (float matched before plain integers).
syn match wispFloat "\<[0-9]\+\.[0-9]\+\>"
syn match wispNumber "\<[0-9]\+\>"

" Strings. Single quotes are literal; double quotes allow ${...} interpolation.
syn region wispStringSingle start=+'+ end=+'+
syn region wispStringDouble start=+"+ skip=+\\"+ end=+"+ contains=wispInterp,wispStringEscape
syn match wispStringEscape "\\." contained
syn region wispInterp matchgroup=wispInterpDelim start=+${+ end=+}+ contained contains=wispKeyword,wispType,wispNamespace,wispBuiltin,wispConstant,wispNumber,wispFloat,wispOperator

" Type-parameter bounds: `comparable` and `numeric` are highlighted only in
" bound position - inside a type-param list AND immediately after a `:`, so
" neither an ordinary annotation like `x: comparable` nor a type parameter
" NAMED comparable/numeric (`fn k[comparable]`) is matched. These are
" contextual words, not keywords, so they stay ordinary identifiers elsewhere.
syn match wispBound "\[[A-Za-z0-9_ ,:]*:\s*\zscomparable\>"
syn match wispBound "\[[A-Za-z0-9_ ,:]*:\s*\zsnumeric\>"

" Operators.
syn match wispOperator "->\|<<\|>>\|&&\|||\|==\|!=\|<=\|>=\|[-+*/%!<>=&|^]"

" Function declaration name.
syn match wispFunction "\<fn\>\s\+\zs[A-Za-z_][A-Za-z0-9_]*"

hi def link wispKeyword Keyword
hi def link wispType Type
hi def link wispNamespace Identifier
hi def link wispBuiltin Function
hi def link wispConstant Constant
hi def link wispComment Comment
hi def link wispNumber Number
hi def link wispFloat Float
hi def link wispStringSingle String
hi def link wispStringDouble String
hi def link wispStringEscape SpecialChar
hi def link wispInterp Special
hi def link wispInterpDelim Delimiter
hi def link wispOperator Operator
hi def link wispFunction Function
hi def link wispBound Type

let b:current_syntax = "wisp"
