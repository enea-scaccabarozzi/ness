package session_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/ladder"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// defaultLadder is the claude ladder of a project that sets nothing under [models].
func defaultLadder(t *testing.T) ladder.Ladder {
	t.Helper()
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"make validate\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	return ladder.New(cfg, "claude")
}

func TestPick(t *testing.T) {
	// Rejects: a Pick that ignores the offset, drops model/effort/index, applies the threshold
	// as <= instead of <, reports a clamp silently, or tags a normal answer as base.
	l := defaultLadder(t)
	b := session.Brief{Text: "do x", Role: "child", Client: "claude", Kind: "implement"}
	cases := []struct {
		name      string
		sel       testkit.FakeSelector
		want      session.Choice
		prefix    string
		suffix    string
		notPrefix bool
	}{
		{"offset", testkit.FakeSelector{Offset: -2, Confidence: 1},
			session.Choice{Tier: "economy", Model: "sonnet", Effort: "medium", Index: 1}, "", "", true},
		{"low confidence", testkit.FakeSelector{Offset: 2, Confidence: 0.79},
			session.Choice{Tier: "standard", Model: "opus", Effort: "medium", Index: 3}, "base: low confidence (0.79)", "", false},
		{"at threshold", testkit.FakeSelector{Offset: 2, Confidence: 0.8},
			session.Choice{Tier: "frontier", Model: "fable", Effort: "medium", Index: 5}, "", "", true},
		{"clamped", testkit.FakeSelector{Offset: 50, Confidence: 1},
			session.Choice{Tier: "apex", Model: "fable", Effort: "high", Index: 6}, "", " (clamped)", true},
		{"clamped down", testkit.FakeSelector{Offset: -50, Confidence: 1},
			session.Choice{Tier: "bulk", Model: "haiku", Effort: "medium", Index: 0}, "", " (clamped)", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := session.Pick(tc.sel, b, l, 0.8)
			if err != nil {
				t.Fatal(err)
			}
			if c.Tier != tc.want.Tier || c.Model != tc.want.Model || c.Effort != tc.want.Effort || c.Index != tc.want.Index {
				t.Fatalf("Pick = %+v, want rung %+v", c, tc.want)
			}
			if !strings.HasPrefix(c.Reason, tc.prefix) || !strings.HasSuffix(c.Reason, tc.suffix) {
				t.Errorf("reason %q: want prefix %q suffix %q", c.Reason, tc.prefix, tc.suffix)
			}
			if tc.notPrefix && strings.HasPrefix(c.Reason, "base:") {
				t.Errorf("reason %q claims a base fallback", c.Reason)
			}
		})
	}
}

func TestPickFallsBackOnSelectorError(t *testing.T) {
	// Rejects: a Pick that refuses (empty Choice) on a selector error, or swallows the error.
	boom := errors.New("selector exploded")
	c, err := session.Pick(testkit.FakeSelector{Err: boom}, session.Brief{Role: "root", Client: "claude"},
		defaultLadder(t), 0.8)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the selector's", err)
	}
	if c.Tier != "standard" || c.Model != "opus" || c.Effort != "medium" || c.Index != 3 ||
		c.Reason != "base: "+boom.Error() {
		t.Fatalf("Choice = %+v", c)
	}
}

func TestFakeSelectorKeepsContract(t *testing.T) {
	// Rejects: a fake whose reason is empty or multi-line, so contract tests would pass vacuously.
	// covers: R-MOD-041
	testkit.PolicyContract(t, testkit.FakeSelector{Offset: 1, Confidence: 0.9})
}
