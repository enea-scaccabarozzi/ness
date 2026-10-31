package stats_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestReadMetaOfEachSubagent(t *testing.T) {
	// Rejects: a meta reader that drops the agent type, model or effort, or fails on the extra
	// keys Claude Code writes (toolUseId, spawnDepth, finished).
	// covers: R-STAT-021
	want := map[string]stats.Meta{
		"agent-a1": {AgentType: "ness-sweep", Model: "sonnet", Effort: "medium"},
		"agent-b2": {AgentType: "ness-verify", Model: "opus", Effort: "high"},
		"agent-c3": {AgentType: "general-purpose", Model: "haiku", Effort: "low"},
	}
	for name, w := range want {
		f, err := os.Open(filepath.Join("testdata", "rung", "s1", "subagents", name+".meta.json"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := stats.ReadMeta(f)
		_ = f.Close()
		if err != nil || got != w {
			t.Errorf("%s: ReadMeta = %+v, %v; want %+v", name, got, err, w)
		}
	}
}

func TestReadMetaWithoutModel(t *testing.T) {
	// Rejects: inventing a model or effort for a subagent launched with defaults, or treating a
	// non-object as a meta.
	// covers: R-STAT-021
	got, err := stats.ReadMeta(strings.NewReader(`{"agentType":"ness-sweep","launchedWithDefaults":true}`))
	if err != nil || got != (stats.Meta{AgentType: "ness-sweep"}) {
		t.Fatalf("ReadMeta = %+v, %v", got, err)
	}
	if _, err := stats.ReadMeta(strings.NewReader(`[1]`)); err == nil {
		t.Fatal("an array is not a meta")
	}
}

func TestRungFixtureCosts(t *testing.T) {
	// Pins the fixture the CLI's --by rung test copies: session 400 cents (opus), subagents 200
	// (sonnet), 100 (opus), 50 (haiku); a session file that already held its subagents' calls
	// would cost more than 400 and the rung split would count them twice.
	// covers: R-STAT-020
	for name, cents := range map[string]int{
		"s1.jsonl": 400, "s1/subagents/agent-a1.jsonl": 200,
		"s1/subagents/agent-b2.jsonl": 100, "s1/subagents/agent-c3.jsonl": 50,
	} {
		f, err := os.Open(filepath.Join("testdata", "rung", name))
		if err != nil {
			t.Fatal(err)
		}
		tr, err := stats.Read(f, time.Time{})
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got, unpriced := tr.Cost(); got != cents || unpriced {
			t.Errorf("%s: cost %d (unpriced %v), want %d", name, got, unpriced, cents)
		}
	}
}
