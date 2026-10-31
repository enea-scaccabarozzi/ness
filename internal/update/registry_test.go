package update_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// project writes <dir>/<name>/.ness/ness.toml and returns <dir>/<name>.
func project(t *testing.T, dir, name string) string {
	t.Helper()
	root := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Join(root, ".ness"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ness", "ness.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRegistryAddListPrune(t *testing.T) {
	// covers: R-UPD-031
	dir := t.TempDir()
	a, b := project(t, dir, "a"), project(t, dir, "b")
	r := update.Registry{Path: filepath.Join(dir, "cfg", "ness", "projects")}
	for _, p := range []string{a, b, a} {
		if err := r.Add(p); err != nil {
			t.Fatal(err)
		}
	}
	if data, err := os.ReadFile(r.Path); err != nil || string(data) != a+"\n"+b+"\n" {
		t.Fatalf("file = %q, %v", data, err)
	}
	if err := os.RemoveAll(filepath.Join(a, ".ness")); err != nil {
		t.Fatal(err)
	}
	got, err := r.List()
	if err != nil || !slices.Equal(got, []string{b}) {
		t.Fatalf("List = %v, %v, want [%s]", got, err, b)
	}
	if data, _ := os.ReadFile(r.Path); string(data) != a+"\n"+b+"\n" {
		t.Fatalf("List wrote the file: %q", data)
	}
	c := project(t, dir, "c")
	if err := r.Add(c); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(r.Path); string(data) != b+"\n"+c+"\n" {
		t.Fatalf("Add did not prune: %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Dir(r.Path)); len(entries) != 1 {
		t.Fatalf("dir = %v, want only projects", entries)
	}
}

func TestRegistryEmptyPath(t *testing.T) {
	if err := (update.Registry{}).Add("/x"); err == nil || !strings.Contains(err.Error(), "machine registry is off") {
		t.Fatalf("Add = %v", err)
	}
	if _, err := (update.Registry{}).List(); err == nil {
		t.Fatal("List: want error")
	}
}

func TestRegistryKeepsAnUnreadableProject(t *testing.T) {
	// covers: R-UPD-031, R-UPD-080
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	dir := t.TempDir()
	a := project(t, dir, "a")
	r := update.Registry{Path: filepath.Join(dir, "projects")}
	if err := r.Add(a); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(a, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(a, 0o750) }) //nolint:errcheck // TempDir cleanup needs it back
	got, err := r.List()
	if err != nil || !slices.Equal(got, []string{a}) {
		t.Fatalf("List = %v, %v: a stat error other than not-exist must keep the project", got, err)
	}
}
