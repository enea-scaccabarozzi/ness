package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/hook"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// spawnFixture is a bound session over a fresh store and the config built from models.
func spawnFixture(t *testing.T, models string) (Deps, bound, *config.Config, *bytes.Buffer) {
	t.Helper()
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"true\"\n" + models))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var stderr bytes.Buffer
	d := Deps{Stderr: &stderr, Now: func() time.Time { return time.Unix(0, 0) }}
	return d, bound{p: &project{root: t.TempDir()}, s: s, c: caller{id: "s-aaaaaaaaaaaa"}}, cfg, &stderr
}

func agentCall(input string) hook.Payload {
	return hook.Payload{ToolName: "Agent", ToolInput: []byte(input)}
}

func TestSpawnGuardSelectorTimeoutJudgesAgainstBase(t *testing.T) {
	// Rejects: a hook that waits on a hung selector past its timeout, or judges a timed-out
	// selector against anything but base (Review Focus).
	// covers: R-HOOK-035
	d, b, cfg, _ := spawnFixture(t, "[models]\npolicy = \"command\"\ncommand = \"sleep 30\"\ncommand_timeout = \"300ms\"\n")
	start := time.Now()
	if err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: search","model":"opus","effort":"medium"}`)); err != nil {
		t.Fatalf("a base-rung call was refused: %v", err)
	}
	if took := time.Since(start); took > 1300*time.Millisecond {
		t.Fatalf("hook took %v with a 300ms selector timeout", took)
	}
	err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: search","model":"fable","effort":"medium"}`))
	if err == nil || !strings.Contains(err.Error(), "standard") {
		t.Fatalf("a call above base: %v", err)
	}
	got, _ := b.s.Spawns("")
	if len(got) != 2 || got[0].Recommended != "standard" || got[0].Verdict != "allowed" {
		t.Fatalf("spawns = %+v", got)
	}
}

func TestSpawnGuardStoreFailureNeverBlocks(t *testing.T) {
	// Rejects: a failed log that blocks an allowed call, swallows the error, or loses a refusal.
	// covers: R-HOOK-033
	d, b, cfg, stderr := spawnFixture(t, "")
	if err := b.s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: search","model":"sonnet","effort":"medium"}`)); err != nil {
		t.Fatalf("allowed call blocked by the log: %v", err)
	}
	if !strings.Contains(stderr.String(), "ness:") {
		t.Fatalf("log error not reported: %q", stderr.String())
	}
	var ee *exitError
	if err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: search","model":"opus","effort":"medium"}`)); !errors.As(err, &ee) {
		t.Fatalf("refusal lost with the log down: %v", err)
	}
}

func TestSpawnGuardModelWithoutEffort(t *testing.T) {
	// Rejects: refusing a model without an effort when the kind's rung has that model, taking an
	// effort for a model the kind's rung does not use, or letting the default effort skip the
	// recommendation check (a selector failing to base still refuses deep without escalate).
	// covers: R-HOOK-034, R-HOOK-031
	d, b, cfg, _ := spawnFixture(t, "")
	for _, in := range []string{
		`{"prompt":"kind: search\nfind x","model":"sonnet"}`,
		`{"prompt":"kind: sweep\nfind x","model":"sonnet"}`,
		`{"subagent_type":"ness-design","prompt":"design x","model":"opus"}`,
	} {
		if err := spawnGuard(d, b, cfg, agentCall(in)); err != nil {
			t.Errorf("%s: refused: %v", in, err)
		}
	}
	if err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: search","model":"opus"}`)); err == nil ||
		!strings.Contains(err.Error(), "is not a rung") {
		t.Errorf("opus for a sonnet kind: %v", err)
	}
	got, _ := b.s.Spawns("")
	if len(got) != 4 || got[0].Requested != "economy" || got[0].Verdict != "allowed" ||
		got[1].Kind != "search" || got[2].Requested != "deep" || got[3].Verdict != "refused" {
		t.Fatalf("spawns = %+v", got)
	}

	d, b, cfg, _ = spawnFixture(t, "[models]\npolicy = \"command\"\ncommand = \"false\"\n")
	if err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: design","model":"opus"}`)); err == nil ||
		!strings.Contains(err.Error(), "above the recommended standard") {
		t.Errorf("deep over a base recommendation: %v", err)
	}
	if err := spawnGuard(d, b, cfg, agentCall(`{"prompt":"kind: design\nescalate: why","model":"opus"}`)); err != nil {
		t.Errorf("escalated deep refused: %v", err)
	}
}
