package session_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestPhasesGolden(t *testing.T) {
	// covers: R-SESS-044
	want := []string{
		"registered", "creating-worktree", "linking-files", "writing-brief", "opening-tab",
		"starting-agent", "sending-brief", "waiting-ready", "ready",
	}
	if !slices.Equal(session.Phases, want) {
		t.Fatalf("Phases = %v", session.Phases)
	}
	if got := session.NextPhases("opening-tab"); !slices.Equal(got, want[4:]) {
		t.Fatalf("NextPhases(opening-tab) = %v", got)
	}
	if got := session.NextPhases("registered"); !slices.Equal(got, want[1:]) {
		t.Fatalf("NextPhases(registered) = %v", got)
	}
}

func TestShownPhase(t *testing.T) {
	// covers: R-SESS-033, R-SESS-085
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	s := store.Session{Phase: "waiting-ready", PhaseAt: now.Add(-59 * time.Second)}
	if got := session.Shown(s, now, time.Minute); got != "waiting-ready" {
		t.Fatalf("young launch shown as %q", got)
	}
	s.PhaseAt = now.Add(-61 * time.Second)
	if got := session.Shown(s, now, time.Minute); got != "stalled launch" {
		t.Fatalf("old launch shown as %q", got)
	}
	s.PhaseAt = now.Add(-time.Minute)
	if got := session.Shown(s, now, time.Minute); got != "stalled launch" {
		t.Fatalf("exactly the timeout shown as %q", got)
	}
	s.PhaseAt = now.Add(-61 * time.Second)
	if got := session.Shown(s, now, 2*time.Minute); got != "waiting-ready" {
		t.Fatalf("with a 120 s timeout shown as %q", got)
	}
	s.ReadyAt = now
	if got := session.Shown(s, now, time.Minute); got != "ready" {
		t.Fatalf("marked ready shown as %q", got)
	}
}

func TestIDsScopeAndPrompt(t *testing.T) {
	// covers: R-SESS-042, R-SESS-043, R-SESS-025
	seen := map[string]bool{}
	for range 1000 {
		id := session.NewID()
		if len(id) != 14 || !strings.HasPrefix(id, "s-") || strings.Trim(id[2:], "0123456789abcdef") != "" || seen[id] {
			t.Fatalf("bad or repeated id %q", id)
		}
		seen[id] = true
	}
	for s, ok := range map[string]bool{
		"sync": true, "a1-b2": true, "Sync_1": false, "1sync": false, "": false,
		strings.Repeat("a", 32): true, strings.Repeat("a", 33): false,
	} {
		if session.ScopeRe.MatchString(s) != ok {
			t.Fatalf("scope %q accepted = %v", s, !ok)
		}
	}
	want := "You are ness session s-aaaaaaaaaaaa (sync). Read docs/sessions/root/sync/brief.md, then load ness-entry."
	if got := session.Prompt("s-aaaaaaaaaaaa", "sync", "docs/sessions/root/sync"); got != want || strings.Contains(got, "\n") {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestCheckVersion(t *testing.T) {
	// covers: R-SESS-002, R-SESS-003
	if err := session.CheckVersion("herdr", "0.8.2", "0.8.2"); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckVersion("herdr", "0.9.0", "0.8.2"); err != nil {
		t.Fatal(err)
	}
	err := session.CheckVersion("herdr", "0.5.0", "0.8.2")
	if err == nil || !strings.Contains(err.Error(), "0.5.0") || !strings.Contains(err.Error(), "0.8.2") {
		t.Fatalf("older herdr: %v", err)
	}
	for v, ok := range map[string]bool{
		"0.8.1": false, "0.8.2": true, "0.8.3": true, "0.9.0": true,
		"1.0.0": false, "1.9.9": false, "0.8.2-rc1": false, "0.8.3-rc1": true, "v0.8.2": true,
	} {
		if err := session.CheckVersion("herdr", v, "0.8.2"); (err == nil) != ok {
			t.Fatalf("herdr %s: err = %v, want accepted = %v", v, err, ok)
		}
	}
	// 1.9.9 above and 3.2.0 here are newer by (minor, patch): only the major compare refuses them.
	if err := session.CheckVersion("claude", "3.2.0", "2.1.294"); err == nil {
		t.Fatal("another major accepted")
	}
	if err := session.CheckVersion("claude", "weird", "2.1.294"); err == nil {
		t.Fatal("unparsable version accepted")
	}
}

func TestShadowing(t *testing.T) {
	// An exact name in any global root refuses, naming both paths; plugin (namespaced) and
	// near names pass.
	// covers: R-SESS-005, R-SESS-006, R-SESS-007
	home := t.TempDir()
	project := filepath.Join(home, "repo", ".agents", "skills")
	for _, d := range []string{filepath.Join(project, "deploy"), filepath.Join(project, "x:deploy"), filepath.Join(project, "review")} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	roots := session.SkillRoots(home)
	want := []string{filepath.Join(home, ".claude", "skills"), "/etc/claude-code/skills", filepath.Join(home, ".agents", "skills"), "/etc/codex/skills"}
	if !slices.Equal(roots, want) {
		t.Fatalf("SkillRoots = %v", roots)
	}
	fakeRoots := []string{filepath.Join(home, "r1"), filepath.Join(home, "r2"), filepath.Join(home, "r3"), filepath.Join(home, "r4")}
	if err := session.CheckShadowing(project, fakeRoots); err != nil {
		t.Fatalf("no globals: %v", err)
	}
	for i, r := range fakeRoots {
		if err := os.MkdirAll(filepath.Join(r, "deploy-prod"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := session.CheckShadowing(project, fakeRoots); err != nil {
			t.Fatalf("near name refused: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(r, "deploy"), 0o750); err != nil {
			t.Fatal(err)
		}
		err := session.CheckShadowing(project, fakeRoots)
		if err == nil || !strings.Contains(err.Error(), filepath.Join(r, "deploy")) || !strings.Contains(err.Error(), filepath.Join(project, "deploy")) {
			t.Fatalf("root %d: %v", i, err)
		}
		if err := os.RemoveAll(filepath.Join(r, "deploy")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(project, "review"), filepath.Join(project, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fakeRoots[0], "linked"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckShadowing(project, fakeRoots); err == nil {
		t.Fatal("symlinked project skill dir not checked")
	}
	if err := os.RemoveAll(filepath.Join(fakeRoots[0], "linked")); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckShadowing(filepath.Join(home, "file"), fakeRoots); err != nil {
		t.Fatalf("missing project skills dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckShadowing(filepath.Join(home, "file"), fakeRoots); err == nil {
		t.Fatal("unreadable project skills dir ignored")
	}
	if err := os.MkdirAll(filepath.Join(fakeRoots[0], "x:deploy"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckShadowing(project, fakeRoots); err != nil {
		t.Fatalf("plugin skill refused: %v", err)
	}
}

func TestAgentName(t *testing.T) {
	// covers: R-SESS-035, R-SESS-036
	if got := session.AgentName("demo-app", "sync", "s-0123456789ab", nil); got != "demo-app-sync" {
		t.Fatalf("AgentName = %q", got)
	}
	long := session.AgentName("demo-app", strings.Repeat("s", 60), "s-0123456789ab", nil)
	if len(long) != 32 || !strings.HasPrefix(long, "demo-app-sss") {
		t.Fatalf("long = %q", long)
	}
	got := session.AgentName("fin-harness", "hotfixes", "s-0123456789ab", []string{"fin-harness-hotfixes"})
	if got != "fin-harness-hotfixes-0" {
		t.Fatalf("collision = %q", got)
	}
	got = session.AgentName("fin-harness", "hotfixes", "s-0123456789ab", []string{"fin-harness-hotfixes", "fin-harness-hotfixes-0"})
	if got != "fin-harness-hotfixes-01" {
		t.Fatalf("double collision = %q", got)
	}
	if got := session.AgentName("Ness", "Sync.1", "s-0123456789ab", nil); got != "ness-sync-1" {
		t.Fatalf("sanitised = %q", got)
	}
	if got := session.AgentName(strings.Repeat("p", 31), "x", "s-0123456789ab", []string{strings.Repeat("p", 31) + "-"}); len(got) != 32 {
		t.Fatalf("suffix within the limit: %q (%d)", got, len(got))
	}
}

func TestTierEffort(t *testing.T) {
	// covers: R-MOD-003
	tier := config.Tier{Claude: "opus/medium", Codex: "gpt-6.1-sol/high"}
	if m, e := session.TierEffort(tier, "claude"); m != "opus" || e != "medium" {
		t.Fatalf("claude = %s/%s", m, e)
	}
	if m, e := session.TierEffort(tier, "codex"); m != "gpt-6.1-sol" || e != "high" {
		t.Fatalf("codex = %s/%s", m, e)
	}
}
