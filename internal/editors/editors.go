// Package editors holds no runtime code. It exists so the editor-asset
// drift-guard test (editors_test.go) has a home in the module: that test reads
// the VSCode and Vim highlighting assets under editors/ and asserts their
// keyword/type/builtin/const word sets stay equal to the compiler's
// authoritative sets, so highlighting can never silently drift from the
// language.
package editors
