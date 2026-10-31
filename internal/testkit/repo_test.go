package testkit_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestModulePath(t *testing.T) {
	// covers: R-QA-043, R-QA-001
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	if first != "module github.com/enea-scaccabarozzi/ness" {
		t.Fatalf("go.mod starts with %q", first)
	}
}

func TestPackageLayout(t *testing.T) {
	// covers: R-QA-002
	allowed := []string{
		".", // the root package embeds CHANGELOG.md (R-SYNC-089)
		"cmd/ness", "internal/cli", "internal/config", "internal/sync", "internal/profile",
		"internal/store", "internal/session", "internal/worker", "internal/probe", "internal/stats",
		"internal/testkit", "internal/hook", "internal/notify", "internal/shell", "internal/ui", "internal/rule", "internal/check", "internal/ladder", "profiles",
	}
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != "../.." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(p, "../../")))
		if !slices.Contains(allowed, dir) && path.Dir(dir) != "internal/atom" {
			t.Errorf("%s is outside the R-QA-002 package layout", dir)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPureGoSQLite(t *testing.T) {
	// covers: R-MSG-002
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "./cmd/ness")
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	deps := string(out)
	if !strings.Contains(deps, "modernc.org/sqlite") {
		t.Fatal("modernc.org/sqlite is not a dependency of cmd/ness")
	}
	for _, cgo := range []string{"github.com/mattn/go-sqlite3", "crawshaw.io/sqlite", "zombiezen.com/go/sqlite"} {
		if strings.Contains(deps, cgo) {
			t.Fatalf("cgo SQLite package %s in the dependency graph", cgo)
		}
	}
}
