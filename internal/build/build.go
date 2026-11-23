// Package build exposes build-time metadata shared by the command binaries.
package build

var Version = "dev"
var Commit = "unknown"
var BuildTime = "unknown"

func String() string { return Version + " commit=" + Commit + " built=" + BuildTime }
