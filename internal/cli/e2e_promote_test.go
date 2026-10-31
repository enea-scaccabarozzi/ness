//go:build e2e

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// promotePrompt is what an operator types into a plain conversation: work that needs a branch,
// with the go-ahead, and no ness command.
const promotePrompt = "Add a file NOTES.md containing the single line `promoted` and commit it. " +
	"Go ahead and start now; do not ask me anything."

func TestRealPlainConversationPromotesItself(t *testing.T) {
	// A plain Claude Code conversation (no NESS_SESSION) in a ness project is asked for work: it
	// promotes itself to a root on a session branch, and its commands then act as that root.
	// covers: R-SESS-114, R-SESS-115, R-HOOK-019, R-BLT-101
	// The promoted root works in its own worktree; the primary checkout stays on main.
	testkit.RequireBudget(t)
	dir := e2eRepo(t, "promote", "claude")
	rt := herdr.Runtime{Exec: cli.SystemExec}
	tab, err := rt.OpenTab(session.TabSpec{Dir: dir, Label: "ness-e2e-promote",
		Env: map[string]string{"PATH": os.Getenv("PATH")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.CloseTab(tab.ID) })
	// The operator's own conversation runs in their permission mode; the e2e repo is throwaway.
	err = rt.StartAgent(session.AgentSpec{Name: "ness-e2e-promote", Pane: tab.Pane, Kind: "claude",
		Args: []string{"--model", "sonnet", "--permission-mode", "bypassPermissions", promotePrompt}})
	if err != nil {
		t.Logf("herdr agent start: %v (the agent may still be working on the prompt)", err)
	}
	root := waitAdopted(t, dir, tab.Pane)
	branch := "session/" + root.Scope + "-" + strings.TrimPrefix(root.ID, "s-")
	if head := strings.TrimSpace(mustRun(t, dir, "git", "symbolic-ref", "--short", "HEAD")); head != "main" {
		t.Fatalf("the primary checkout moved to %q; it must stay on main", head)
	}
	if _, serr := os.Stat(filepath.Join(dir, root.Worktree, "docs", "sessions", root.Scope, "brief.md")); serr != nil {
		t.Fatalf("no brief in the root's worktree record: %v", serr)
	}
	t.Logf("observed: conversation promoted itself as %s scope %s in %s", root.ID, root.Scope, root.Worktree)
	waitCommitted(t, dir, branch, "NOTES.md")
	if out, _ := run(t, dir, "git", "status", "--porcelain"); strings.Contains(out, "NOTES.md") {
		t.Fatalf("NOTES.md written in the primary checkout:\n%s", out)
	}
	t.Logf("observed: NOTES.md committed on %s\n%s", branch, paneText(t, tab.Pane))
}

// waitAdopted waits for the root the conversation in pane registers.
func waitAdopted(t *testing.T, dir, pane string) sess {
	t.Helper()
	for {
		for _, s := range sessions(t, dir) {
			if s.Pane == pane {
				return s
			}
		}
		select {
		case <-t.Context().Done():
			t.Fatalf("no root adopted from pane %s\n%s", pane, paneText(t, pane))
		case <-time.After(5 * time.Second):
		}
		if time.Now().After(deadline(t)) {
			t.Fatalf("no root adopted from pane %s within the budget\n%s", pane, paneText(t, pane))
		}
	}
}

// waitCommitted waits until file is committed on branch.
func waitCommitted(t *testing.T, dir, branch, file string) {
	t.Helper()
	for {
		if out, err := run(t, dir, "git", "log", "--format=%h", branch, "--", file); err == nil && out != "" {
			return
		}
		if time.Now().After(deadline(t)) {
			t.Fatalf("%s not committed on %s within the budget", file, branch)
		}
		select {
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-time.After(5 * time.Second):
		}
	}
}

// deadline is the test's deadline from startClock.
func deadline(t *testing.T) time.Time {
	t.Helper()
	if dl, ok := deadlines.Load(t.Name()); ok {
		return dl.(time.Time) //nolint:forcetypeassert // only startClock stores
	}
	return time.Now().Add(testBudget)
}
