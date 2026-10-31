// Package testkit is the shared test setup: hermetic git repositories and custom commands for
// CLI scripts. Later plans add the atom fakes here.
package testkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/rogpeppe/go-internal/testscript"
)

const gitconfig = `[user]
	name = ness test
	email = test@example.invalid
[init]
	defaultBranch = main
[commit]
	gpgsign = false
`

// Setup makes a script hermetic (R-QA-021): it refuses a work dir outside the temp dir, drops
// inherited GIT_* variables, points HOME inside $WORK and creates a git repository at
// $WORK/repo on branch main with one empty commit.
func Setup(e *testscript.Env) error {
	if err := insideTemp(e.WorkDir); err != nil {
		return err
	}
	vars := make([]string, 0, len(e.Vars))
	for _, kv := range e.Vars {
		if !strings.HasPrefix(kv, "GIT_") {
			vars = append(vars, kv)
		}
	}
	e.Vars = vars
	home := filepath.Join(e.WorkDir, ".home")
	e.Setenv("HOME", home)
	// launch.max_children defaults to half the cores: scripts open several children on any runner.
	e.Setenv("GOMAXPROCS", "8")
	e.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(home, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil {
		return err
	}
	repo := filepath.Join(e.WorkDir, "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		return err
	}
	ctx := context.Background()
	for _, cmd := range []*exec.Cmd{
		exec.CommandContext(ctx, "git", "init", "-q"),
		exec.CommandContext(ctx, "git", "commit", "-q", "--allow-empty", "-m", "init"),
	} {
		cmd.Dir, cmd.Env = repo, e.Vars
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %w\n%s", cmd.Args, err, out)
		}
	}
	fakes := filepath.Join(e.WorkDir, ".fakes")
	if err := WriteFakeDefaults(fakes); err != nil {
		return err
	}
	e.Setenv("NESS_TEST_FAKES", fakes)
	goCacheEnv(e)
	return nil
}

// goCacheEnv hands the outer process's Go caches to a script, so golangci-lint in a fixture
// module reuses them instead of compiling the standard library cold under $WORK.
func goCacheEnv(e *testscript.Env) {
	for _, k := range []string{"GOCACHE", "GOMODCACHE"} {
		e.Setenv(k, goEnv(k))
	}
}

var goEnvCache sync.Map // variable -> value, from one `go env` per process

// goEnv is the outer process's value of a Go environment variable.
func goEnv(name string) string {
	if v, ok := goEnvCache.Load(name); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	v := os.Getenv(name)
	if v == "" {
		cmd := exec.CommandContext(context.Background(), "go", "env", name) //nolint:gosec // fixed go subcommand
		out, err := cmd.Output()
		if err == nil {
			v = strings.TrimSpace(string(out))
		}
	}
	goEnvCache.Store(name, v)
	return v
}

func insideTemp(dir string) error {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return err
	}
	abs, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, tmp+string(filepath.Separator)) {
		return fmt.Errorf("work dir %s is outside %s; refusing to run git there", abs, tmp)
	}
	return nil
}

// Cmds returns the custom script commands:
//
//	exits CODE CMD [ARGS...]  run CMD; fail unless it exits with CODE
//	snap NAME [DIR]           record a hash of the tree under DIR (default .), .git excluded
//	samesnap NAME [DIR]       fail unless the tree matches snapshot NAME (negatable)
//	islink PATH TARGET        fail unless PATH is a symlink whose target is TARGET
//	store SUB ...             drive the store through its API (see storeCmd)
//	onejson                   fail unless the last stdout is one JSON object with "schema": 1
func Cmds() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"exits":    exits,
		"snap":     snap,
		"samesnap": samesnap,
		"islink":   islink,
		"store":    storeCmd,
		"fake":     fakeCmd,
		"onejson":  onejson,
	}
}

// onejson checks R-CFG-032/037: the last command's stdout parses as exactly one JSON object
// carrying "schema": 1, with nothing after it.
func onejson(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: onejson")
	}
	dec := json.NewDecoder(strings.NewReader(ts.ReadFile("stdout")))
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		ts.Fatalf("stdout is not a JSON object: %v", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		ts.Fatalf("stdout holds more than one JSON value")
	}
	if v, ok := obj["schema"].(float64); !ok || v != 1 {
		ts.Fatalf(`"schema" = %v, want 1`, obj["schema"])
	}
}

func exits(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: exits CODE CMD [ARGS...]")
	}
	want, err := strconv.Atoi(args[0])
	ts.Check(err)
	got := 0
	if err := ts.Exec(args[1], args[2:]...); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			ts.Fatalf("%s: %v", args[1], err)
		}
		got = ee.ExitCode()
	}
	if got != want {
		ts.Fatalf("%s exited %d, want %d", args[1], got, want)
	}
}

func snapDir(ts *testscript.TestScript, args []string) (name, dir string) {
	if len(args) < 1 || len(args) > 2 {
		ts.Fatalf("usage: snap|samesnap NAME [DIR]")
	}
	dir = "."
	if len(args) == 2 {
		dir = args[1]
	}
	return args[0], ts.MkAbs(dir)
}

func snap(ts *testscript.TestScript, _ bool, args []string) {
	name, dir := snapDir(ts, args)
	h, err := treeHash(dir)
	ts.Check(err)
	ts.Setenv("SNAP_"+name, h)
}

func samesnap(ts *testscript.TestScript, neg bool, args []string) {
	name, dir := snapDir(ts, args)
	h, err := treeHash(dir)
	ts.Check(err)
	if same := h == ts.Getenv("SNAP_"+name); same == neg {
		ts.Fatalf("tree %s: same as snapshot %s = %v", dir, name, same)
	}
}

func islink(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: islink PATH TARGET")
	}
	got, err := os.Readlink(ts.MkAbs(args[0]))
	ts.Check(err)
	if got != args[1] {
		ts.Fatalf("%s links to %q, want %q", args[0], got, args[1])
	}
}

// treeHash hashes paths, types, file bytes and link targets under dir, skipping .git.
func treeHash(dir string) (sum string, err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	h := sha256.New()
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		h.Write([]byte(rel + "\x00" + d.Type().String() + "\x00"))
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			h.Write([]byte(target))
		case d.Type().IsRegular():
			b, err := root.ReadFile(rel)
			if err != nil {
				return err
			}
			h.Write(b)
		}
		h.Write([]byte{0})
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}
