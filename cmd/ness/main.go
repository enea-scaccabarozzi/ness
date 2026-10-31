// Command ness installs and runs an AI coding-agent harness inside one project.
package main

import (
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

// version is the release version, set at release time with -ldflags "-X main.version=<version>";
// a binary built from a release tag takes the tag (R-QA-046).
var version = "0.1.0"

// releaseAndBuild returns the release version (the -ldflags value unless the go command stamped
// a release tag) and the build: the commit the binary was built from, ".dirty" when the tree had
// changes, "" without version control information (R-QA-046).
func releaseAndBuild(set string, bi *debug.BuildInfo) (release, build string) {
	if bi == nil {
		return set, ""
	}
	release = set
	if v := bi.Main.Version; strings.HasPrefix(v, "v") && !strings.ContainsAny(v, "-+") { // a tag, not a pseudo-version
		release = strings.TrimPrefix(v, "v")
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch {
		case s.Key == "vcs.revision":
			rev = s.Value[:min(12, len(s.Value))]
		case s.Key == "vcs.modified" && s.Value == "true":
			dirty = ".dirty"
		}
	}
	if rev == "" {
		return release, ""
	}
	return release, rev + dirty
}

func main() {
	bi, _ := debug.ReadBuildInfo() // nil without build information
	release, build := releaseAndBuild(version, bi)
	os.Exit(cli.Main(os.Args[1:], cli.Deps{
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		Version:      release,
		Build:        build,
		Session:      os.Getenv("NESS_SESSION"),           // the design's identity variable (R-SESS-037)
		Conversation: os.Getenv("CLAUDE_CODE_SESSION_ID"), // an adopted root's identity (R-SESS-114)
		Now:          time.Now,
		Exec:         cli.SystemExec,
		GOOS:         runtime.GOOS,
		TTY:          isTerminal(os.Stdin),
		Term:         ui.Detect(os.Stdin, os.Stdout),
	}))
}

// isTerminal reports whether f is a character device other than /dev/null (a terminal).
// Linux only in v1 (R-SESS-015).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	devnull, err := os.Stat(os.DevNull)
	return err == nil && !os.SameFile(fi, devnull)
}
