// Package version holds the wisp compiler version.
package version

// Number is the wisp compiler version string. It defaults to a development
// value and is overridden at release time with -ldflags -X, so it is a var.
var Number = "0.0.0-dev"
