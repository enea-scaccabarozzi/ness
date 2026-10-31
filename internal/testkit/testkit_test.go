package testkit_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSetupIsHermetic(t *testing.T) {
	// covers: R-QA-021
	work := t.TempDir()
	env := &testscript.Env{WorkDir: work, Vars: []string{"PATH=" + os.Getenv("PATH"), "GIT_DIR=/nonexistent"}}
	if err := testkit.Setup(env); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(env.Vars, func(kv string) bool { return strings.HasPrefix(kv, "GIT_DIR=") }) {
		t.Fatal("GIT_DIR leaked into the script environment")
	}
	if _, err := os.Stat(filepath.Join(work, "repo", ".git")); err != nil {
		t.Fatalf("no repository: %v", err)
	}
	if err := testkit.Setup(&testscript.Env{WorkDir: "/"}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("a work dir outside the temp dir was accepted: %v", err)
	}
}
