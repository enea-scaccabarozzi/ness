package ladder_test

import (
	"math"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/ladder"
)

const minimal = "ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"make validate\"\n"

// build loads a validated config with extra appended and returns its ladder for client.
func build(t *testing.T, extra, client string) ladder.Ladder {
	t.Helper()
	f, err := config.Parse([]byte(minimal + extra))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	return ladder.New(cfg, client)
}

func TestShape(t *testing.T) {
	// Rejects: Ceiling left at 0 when unset, Base not resolved to its index, rungs in map order.
	l := build(t, "", "claude")
	if len(l.Rungs) != 7 || l.Base != 3 || l.Ceiling != 6 {
		t.Fatalf("ladder = %+v", l)
	}
	if l.Rungs[0] != (ladder.Rung{Name: "bulk", Model: "haiku", Effort: "medium"}) ||
		l.Rungs[6] != (ladder.Rung{Name: "apex", Model: "fable", Effort: "high"}) {
		t.Fatalf("ends = %+v, %+v", l.Rungs[0], l.Rungs[6])
	}
}

func TestAt(t *testing.T) {
	// Rejects: an offset taken from index 0 instead of base, an index panic on a huge offset,
	// a clamp that ignores the ceiling or reports true when it clamped, a codex client served
	// claude ids, and a ladder that ignores the project's base or own rung order.
	cases := []struct {
		name, extra, client string
		offset              int
		want                ladder.Rung
		ok                  bool
	}{
		{"base", "", "claude", 0, ladder.Rung{Name: "standard", Model: "opus", Effort: "medium"}, true},
		{"down two", "", "claude", -2, ladder.Rung{Name: "economy", Model: "sonnet", Effort: "medium"}, true},
		{"up two", "", "claude", 2, ladder.Rung{Name: "frontier", Model: "fable", Effort: "medium"}, true},
		{"over top", "", "claude", 9, ladder.Rung{Name: "apex", Model: "fable", Effort: "high"}, false},
		{"under bottom", "", "claude", -9, ladder.Rung{Name: "bulk", Model: "haiku", Effort: "medium"}, false},
		{"huge", "", "claude", 100, ladder.Rung{Name: "apex", Model: "fable", Effort: "high"}, false},
		{"huge negative", "", "claude", -100, ladder.Rung{Name: "bulk", Model: "haiku", Effort: "medium"}, false},
		{"max int", "", "claude", math.MaxInt, ladder.Rung{Name: "apex", Model: "fable", Effort: "high"}, false},
		{"min int", "", "claude", math.MinInt, ladder.Rung{Name: "bulk", Model: "haiku", Effort: "medium"}, false},
		{"ceiling", "[models]\nceiling = \"frontier\"\n", "claude", 3,
			ladder.Rung{Name: "frontier", Model: "fable", Effort: "medium"}, false},
		{"at ceiling", "[models]\nceiling = \"frontier\"\n", "claude", 2,
			ladder.Rung{Name: "frontier", Model: "fable", Effort: "medium"}, true},
		{"economy base down", "[models]\nbase = \"economy\"\n", "claude", -2,
			ladder.Rung{Name: "bulk", Model: "haiku", Effort: "medium"}, false},
		{"economy base up", "[models]\nbase = \"economy\"\n", "claude", 2,
			ladder.Rung{Name: "standard", Model: "opus", Effort: "medium"}, true},
		{"codex", "client = \"codex\"\nexperimental = [\"codex\"]\n", "codex", 0,
			ladder.Rung{Name: "standard", Model: "gpt-6.1-sol", Effort: "medium"}, true},
		{"codex down", "", "codex", -2, ladder.Rung{Name: "economy", Model: "gpt-6-luna", Effort: "medium"}, true},
		{"own order", "[models]\nladder = [\"economy\", \"standard\", \"apex\"]\n", "claude", 1,
			ladder.Rung{Name: "apex", Model: "fable", Effort: "high"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := build(t, tc.extra, tc.client).At(tc.offset)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("At(%d) = %+v, %v; want %+v, %v", tc.offset, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestIndexStepFind(t *testing.T) {
	// Rejects: Step that walks off either end or past the ceiling, Step that ignores the name
	// and moves from base, Find matching the model alone, Index answering for unknown names.
	l := build(t, "", "claude")
	if i, ok := l.Index("deep"); i != 4 || !ok {
		t.Errorf("Index(deep) = %d, %v", i, ok)
	}
	if _, ok := l.Index("ultra"); ok {
		t.Error("Index(ultra) found")
	}
	steps := []struct {
		from  string
		delta int
		want  string
		ok    bool
	}{
		{"standard", 1, "deep", true},
		{"lean", -1, "economy", true},
		{"apex", 1, "apex", false},
		{"bulk", -1, "bulk", false},
	}
	for _, s := range steps {
		if r, ok := l.Step(s.from, s.delta); r.Name != s.want || ok != s.ok {
			t.Errorf("Step(%s, %d) = %+v, %v; want %s, %v", s.from, s.delta, r, ok, s.want, s.ok)
		}
	}
	capped := build(t, "[models]\nceiling = \"frontier\"\n", "claude")
	if r, ok := capped.Step("frontier", 1); r.Name != "frontier" || ok {
		t.Errorf("Step at ceiling = %+v, %v", r, ok)
	}
	if i, ok := l.Find("opus", "high"); i != 4 || !ok {
		t.Errorf("Find(opus, high) = %d, %v", i, ok)
	}
	if _, ok := l.Find("opus", "max"); ok {
		t.Error("Find(opus, max) found")
	}
}
