// Package buildinfo holds build-time metadata injected via -ldflags.
package buildinfo

// Version is the semver string set at build time (e.g. "0.2.0").
// Falls back to "dev" when built without -ldflags.
var Version = "dev"

// GitCommit is the short git SHA set at build time.
var GitCommit = "unknown"
