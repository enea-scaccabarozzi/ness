package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestOwnedWorktrees(t *testing.T) {
	// A closed session whose worktree was kept still owns it; a plainly closed one does not.
	// covers: R-DOC-014
	owned := ownedWorktrees([]store.Session{
		{Worktree: ".worktrees/open-1"},
		{Worktree: ".worktrees/kept-2", Closed: true, KeepReason: "review"},
		{Worktree: ".worktrees/gone-3", Closed: true},
	})
	if !owned["open-1"] || !owned["kept-2"] || owned["gone-3"] {
		t.Fatalf("owned = %v", owned)
	}
}

func TestCheckWorkerClients(t *testing.T) {
	// A client without worker support is a doctor line naming it, when it is configured or an open
	// session runs on it, with the sessions it leaves unwoken (M5, R-WRK-010, R-WRK-091).
	// covers: R-WRK-010, R-WRK-091, R-DOC-010
	path := filepath.Join(t.TempDir(), "ness.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	add := func(id, client string) {
		if err := s.AddLaunch(store.Launch{ID: id, Parent: store.Operator, Scope: id, Title: id, Client: client}, now); err != nil {
			t.Fatal(err)
		}
	}
	add("a", "claude")
	e := &doctorEnv{s: s, cfg: &config.Config{Client: "claude"}}
	if c := checkWorkerClients(e); c.Status != "ok" || c.Detail != "the worker delivers to claude" {
		t.Fatalf("claude only: %+v", c)
	}
	add("b", "codex")
	c := checkWorkerClients(e)
	if c.Status != "warn" || c.Detail != "codex has no worker support, not woken: b" {
		t.Fatalf("codex open: %+v", c)
	}
	if err := s.CloseSession("b"); err != nil {
		t.Fatal(err)
	}
	if c := checkWorkerClients(e); c.Status != "ok" {
		t.Fatalf("codex closed: %+v", c)
	}
	e.cfg.Client = "codex"
	if c := checkWorkerClients(e); c.Status != "warn" || c.Detail != "codex has no worker support" {
		t.Fatalf("codex configured: %+v", c)
	}
}
