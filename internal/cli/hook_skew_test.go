package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestPreToolBlocksOnUnresolvedProfiles(t *testing.T) {
	// Rejects: a session whose profiles do not resolve running its tools unguarded (fail open),
	// or blocking the ness command that repairs it.
	// covers: R-UPD-020, R-UPD-021
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"true\"\nprofiles = [\"nope\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	b := bound{p: &project{root: t.TempDir(), file: f}, s: s, c: caller{id: "s-aaaaaaaaaaaa"}}
	run := func(payload string) error {
		return preTool(Deps{Stdin: strings.NewReader(payload), Stderr: &bytes.Buffer{}}, b)
	}
	err = run(`{"tool_name":"Edit","tool_input":{"file_path":"x","old_string":"a","new_string":"b"}}`)
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != ExitUsage ||
		!strings.HasPrefix(err.Error(), "hooks cannot run in this session: ") ||
		!strings.Contains(err.Error(), `"nope"`) || !strings.HasSuffix(err.Error(), "; fix: correct .ness/ness.toml") {
		t.Fatalf("edit under unresolved profiles: %v", err)
	}
	if err := run(`{"tool_name":"Bash","tool_input":{"command":"ness doctor"}}`); err != nil {
		t.Fatalf("ness doctor blocked: %v", err)
	}
}
