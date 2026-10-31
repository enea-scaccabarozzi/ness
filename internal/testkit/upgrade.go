package testkit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rogpeppe/go-internal/txtar"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

// upgradeDir holds one directory per stable release, built by scripts/upgrade-fixture.sh
// (R-UPD-094). The release check (.github/scripts/release-check.sh) looks for the same path.
var upgradeDir = filepath.Join("internal", "cli", "testdata", "upgrade")

// PatchRule fails when manifest is a patch release over fixtureVersion (same major.minor, a
// higher patch) and the store schema moved from fixtureSchema to schema (R-UPD-001).
func PatchRule(manifest, fixtureVersion string, fixtureSchema, schema int) error {
	c, err := nsync.Compare(manifest, fixtureVersion)
	if err != nil {
		return err
	}
	if c > 0 && minor(manifest) == minor(fixtureVersion) && fixtureSchema != schema {
		return fmt.Errorf("patch release %s over %s adds a store migration (schema %d → %d)",
			manifest, fixtureVersion, fixtureSchema, schema)
	}
	return nil
}

// minor is the major.minor of an X.Y.Z version.
func minor(v string) string {
	i := strings.LastIndex(v, ".")
	return v[:max(i, 0)]
}

// upgradeFixture unpacks a release fixture into $WORK/repo, commits it and loads its store dump
// with no migration, then sets FIXTURE_VERSION (X.Y.Z) and FIXTURE_SCHEMA (the dump's schema).
func upgradeFixture(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: upgradefixture latest|vX.Y.Z")
	}
	root, err := moduleRoot()
	ts.Check(err)
	base := filepath.Join(root, upgradeDir)
	tag := args[0]
	if tag == "latest" {
		tag, err = latestFixture(base)
		ts.Check(err)
	}
	dir := filepath.Join(base, tag)
	ar, err := txtar.ParseFile(filepath.Join(dir, "tree.txtar"))
	ts.Check(err)
	repo := filepath.Join(ts.Getenv("WORK"), "repo")
	ts.Check(unpack(repo, ar))
	ts.Check(ts.Exec("git", "-C", repo, "add", "-A"))
	ts.Check(ts.Exec("git", "-C", repo, "commit", "-q", "-m", "upgrade fixture "+tag))
	dump, err := os.ReadFile(filepath.Join(dir, "state.sql")) //nolint:gosec // a fixture of the module
	ts.Check(err)
	path, err := store.Locate(repo)
	ts.Check(err)
	ts.Check(os.MkdirAll(filepath.Dir(path), 0o750))
	schema, err := loadDump(path, string(dump))
	ts.Check(err)
	ts.Setenv("FIXTURE_VERSION", strings.TrimPrefix(tag, "v"))
	ts.Setenv("FIXTURE_SCHEMA", strconv.Itoa(schema))
}

// unpack writes the archive's files under repo and makes each "symlink PATH TARGET" line of
// its comment, the form the fixture script gives symlinks.
func unpack(repo string, ar *txtar.Archive) (err error) {
	r, err := os.OpenRoot(repo)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	for _, f := range ar.Files {
		if err := r.MkdirAll(filepath.Dir(f.Name), 0o750); err != nil {
			return err
		}
		if err := r.WriteFile(f.Name, f.Data, 0o600); err != nil {
			return err
		}
	}
	for line := range strings.Lines(string(ar.Comment)) {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "symlink" {
			return fmt.Errorf("tree.txtar comment line %q is not symlink PATH TARGET", line)
		}
		if err := r.MkdirAll(filepath.Dir(fields[1]), 0o750); err != nil {
			return err
		}
		if err := r.Symlink(fields[2], fields[1]); err != nil {
			return err
		}
	}
	return nil
}

// loadDump creates the store at path from an SQL dump with raw database/sql, so the dump's
// schema stays as the old release wrote it, and returns that schema version.
func loadDump(path, dump string) (schema int, err error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	ctx := context.Background()
	if _, err = db.ExecContext(ctx, dump); err != nil {
		return 0, err
	}
	err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schema)
	return schema, err
}

// latestFixture is the newest vX.Y.Z directory under base.
func latestFixture(base string) (string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	best := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if best == "" {
			best = e.Name()
			continue
		}
		c, err := nsync.Compare(e.Name(), best)
		if err != nil {
			return "", err
		}
		if c > 0 {
			best = e.Name()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no upgrade fixture under %s", base)
	}
	return best, nil
}

// patchRule runs PatchRule on the release-please manifest's version, the loaded fixture and
// this binary's schema.
func patchRule(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: patchrule")
	}
	root, err := moduleRoot()
	ts.Check(err)
	b, err := os.ReadFile(filepath.Join(root, ".release-please-manifest.json")) //nolint:gosec // the module's manifest
	ts.Check(err)
	var manifest map[string]string
	ts.Check(json.Unmarshal(b, &manifest))
	fs, err := strconv.Atoi(ts.Getenv("FIXTURE_SCHEMA"))
	if err != nil {
		ts.Fatalf("FIXTURE_SCHEMA: %v (run upgradefixture first)", err)
	}
	ts.Check(PatchRule(manifest["."], ts.Getenv("FIXTURE_VERSION"), fs, store.Version()))
}
