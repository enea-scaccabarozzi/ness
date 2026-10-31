// Package ness holds what the binary embeds from the repository root: the release notes
// (R-SYNC-089). Everything else lives under internal.
package ness

import _ "embed"

// Changelog is CHANGELOG.md as built into this binary; sync prints the versions between the
// lock's and the binary's from it (R-SYNC-032).
//
//go:embed CHANGELOG.md
var Changelog string
