package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestConcurrentWorktreesAllLand(t *testing.T) {
	// 20 parallel real launches lost two worktrees: one launch's `git worktree prune` deleted
	// another's half-made .git/worktrees entry. The per-repository lock serializes prune and add.
	// support for R-SESS-112, which the e2e launch test covers
	run := func(dir, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(context.Background(), name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return out, fmt.Errorf("%s %v: %w: %s", name, args, err, out)
		}
		return out, nil
	}
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.email=t@x", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if _, err := run(repo, "git", args...); err != nil {
			t.Fatal(err)
		}
	}
	l := &Launcher{Primary: repo, Exec: run}
	g := Git{run}
	// Closes (removeWorktree prunes) and integrate's throwaway adds run alongside the launches.
	const m = 4
	for i := range m {
		if _, err := run(repo, "git", "worktree", "add", "-q", "--detach", filepath.Join(repo, ".worktrees", fmt.Sprintf("c%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	const n = 16
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := &store.Session{Base: "main", Branch: fmt.Sprintf("session/p%d", i), Worktree: fmt.Sprintf(".worktrees/p%d", i)}
			errs[i] = l.createWorktree(s, Request{})
		}()
	}
	closeErrs := make([]error, m)
	for i := range m {
		wg.Add(2)
		go func() {
			defer wg.Done()
			closeErrs[i] = removeWorktree(g, CloseSpec{Primary: repo, Sess: store.Session{Worktree: fmt.Sprintf(".worktrees/c%d", i)}})
		}()
		go func() {
			defer wg.Done()
			is := IntegrateSpec{Repo: repo}
			tmp := filepath.Join(repo, ".git", "ness", fmt.Sprintf("tmp-%d", i))
			if err := worktreeOp(g, is, "add", "-q", "--detach", tmp, "main"); err != nil {
				t.Errorf("integrate add %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
	for i, err := range closeErrs {
		if err != nil {
			t.Errorf("close %d: %v", i, err)
		}
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("worktree %d: %v", i, err)
		}
		if _, err := os.Stat(filepath.Join(repo, ".worktrees", fmt.Sprintf("p%d", i), ".git")); err != nil {
			t.Errorf("worktree %d missing: %v", i, err)
		}
	}
	for i := range m {
		if _, err := os.Stat(filepath.Join(repo, ".git", "ness", fmt.Sprintf("tmp-%d", i), ".git")); err != nil {
			t.Errorf("integrate worktree %d missing: %v", i, err)
		}
	}
}
