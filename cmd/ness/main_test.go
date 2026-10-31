package main

import (
	"os"
	"runtime/debug"
	"testing"
)

// Rejects a binary that reports a hardcoded version with no build: a build from a commit past the
// tag must name that commit, a tag build takes the tag, and a build without version control keeps
// the -ldflags release version.
func TestReleaseAndBuildComeFromTheBuild(t *testing.T) {
	// covers: R-QA-046
	vcs := []debug.BuildSetting{
		{Key: "vcs", Value: "git"},
		{Key: "vcs.revision", Value: "7fcaae9122ee9097c15de9ec157c364719d0df3f"},
		{Key: "vcs.modified", Value: "true"},
	}
	for _, c := range []struct {
		name           string
		bi             *debug.BuildInfo
		release, build string
	}{
		{"pseudo-version", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1-0.20261029171450-7fcaae9122ee+dirty"}, Settings: vcs}, "0.1.0", "7fcaae9122ee.dirty"},
		{"clean commit", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs[:2]}, "0.1.0", "7fcaae9122ee"},
		{"release tag", &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}}, "0.2.0", ""},
		{"no vcs", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "0.1.0", ""},
		{"no build info", nil, "0.1.0", ""},
	} {
		if r, b := releaseAndBuild("0.1.0", c.bi); r != c.release || b != c.build {
			t.Errorf("%s: releaseAndBuild = %q, %q; want %q, %q", c.name, r, b, c.release, c.build)
		}
	}
}

func TestIsTerminalRejectsDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Fatal("/dev/null reported as a terminal")
	}
}
