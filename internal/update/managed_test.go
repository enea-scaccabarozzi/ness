package update_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestManagedBy(t *testing.T) {
	// covers: R-UPD-045
	env := map[string]string{"GOPATH": "/g", "MISE_DATA_DIR": "/m"}
	get := func(k string) string { return env[k] }
	for bin, want := range map[string]string{
		"/m/installs/github-enea-scaccabarozzi-ness/0.1.0/ness": "mise use -g github:enea-scaccabarozzi/ness@0.2.0",
		// rejects: checking the default mise dir even when MISE_DATA_DIR is set
		"/h/.local/share/mise/installs/github-enea-scaccabarozzi-ness/0.1.0/ness": "",
		"/g/bin/ness":        "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0",
		"/h/go/bin/ness":     "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0", // rejects: ~/go/bin missed when GOPATH is set
		"/h/.local/bin/ness": "",                                                              // rejects: everything treated as managed
	} {
		got, ok := update.ManagedBy(bin, "0.2.0", get, "/h")
		if got != want || ok != (want != "") {
			t.Errorf("%s = %q,%v want %q", bin, got, ok, want)
		}
	}
	// rejects: no default when MISE_DATA_DIR is unset
	got, ok := update.ManagedBy("/h/.local/share/mise/installs/x/ness", "0.2.0", func(string) string { return "" }, "/h")
	if !ok || !strings.HasPrefix(got, "mise use -g") {
		t.Errorf("default mise dir = %q,%v", got, ok)
	}
	// rejects: $GOBIN ignored (rule names $GOBIN, $GOPATH/bin and ~/go/bin)
	gobin := func(k string) string { return map[string]string{"GOBIN": "/gb"}[k] }
	got, ok = update.ManagedBy("/gb/ness", "0.2.0", gobin, "/h")
	if !ok || got != "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0" {
		t.Errorf("GOBIN = %q,%v", got, ok)
	}
	// rejects: prefix match on a sibling directory name
	if got, ok := update.ManagedBy("/gbx/ness", "0.2.0", gobin, "/h"); ok {
		t.Errorf("/gbx/ness = %q, want not managed", got)
	}
}

func TestManagedBySymlinkedDirs(t *testing.T) {
	// covers: R-UPD-045 — the binary is a resolved path, so a symlinked $GOBIN, $GOPATH or
	// $MISE_DATA_DIR must be resolved too
	root, err := filepath.EvalSymlinks(t.TempDir()) // the binary below must be a resolved path
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ key, sub, want string }{
		{"GOBIN", "", "go install"},
		{"GOPATH", "bin", "go install"},
		{"MISE_DATA_DIR", "installs/x", "mise use -g"},
	} {
		real := filepath.Join(root, c.key, "real")
		if err := os.MkdirAll(filepath.Join(real, c.sub), 0o750); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, c.key, "link")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		get := func(k string) string { return map[string]string{c.key: link}[k] }
		// rejects: comparing the resolved binary against the unresolved directory
		got, ok := update.ManagedBy(filepath.Join(real, c.sub, "ness"), "0.2.0", get, "/h")
		if !ok || !strings.HasPrefix(got, c.want) {
			t.Errorf("%s symlinked = %q,%v want %s", c.key, got, ok, c.want)
		}
	}
}
