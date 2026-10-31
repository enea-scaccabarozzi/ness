package jev_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/jev"
	"github.com/enea-scaccabarozzi/ness/internal/config"
)

const (
	ruleUp   = "novel/irreversible without a check"
	ruleDown = "mechanical with a check"
)

func TestCombine(t *testing.T) {
	// Each row names the wrong implementation it rejects. Kinds are config.DefaultKinds
	// (search -2, run -2, implement 0, review +1, design +1); weights are DefaultWeights.
	cases := []struct {
		name   string
		a      jev.Answers
		offset int
		rule   string // "" means no step and neither rule in the reason
	}{
		// Rejects: stepping search below its own offset (search/run are already the floor kinds).
		{"search mechanical with check stays -2", jev.Answers{Kind: "search", Mechanical: 0.9, Check: 0.9}, -2, ""},
		// Rejects: excluding only search from the down step and not run.
		{"run mechanical with check stays -2", jev.Answers{Kind: "run", Mechanical: 0.9, Check: 0.9}, -2, ""},
		// Rejects: a missing down step, or a down step for implement.
		{"implement mechanical with check -1", jev.Answers{Kind: "implement", Mechanical: 0.9, Check: 0.9}, -1, ruleDown},
		// Rejects: a missing up step on novelty.
		{"implement novel no check +1", jev.Answers{Kind: "implement", Novelty: 0.9, Check: 0.1}, 1, ruleUp},
		// Rejects: an up step that ignores the check (a check present cancels novelty).
		{"implement novel with check stays 0", jev.Answers{Kind: "implement", Novelty: 0.9, Check: 0.5}, 0, ""},
		// Rejects: reading only Novelty and not the Design noul for the up step.
		{"implement design choice no check +1", jev.Answers{Kind: "implement", Design: 0.9, Check: 0.1}, 1, ruleUp},
		// Rejects: an extra step for design work, which already sits at +1.
		{"design novel stays +1", jev.Answers{Kind: "design", Novelty: 0.9, Design: 0.9, Check: 0.1}, 1, ""},
		// Rejects: blast ignored, or blast gated on novelty.
		{"review irreversible no check +2", jev.Answers{Kind: "review", Blast: 1, Check: 0.1}, 2, ruleUp},
		// Rejects: blast up despite a check.
		{"review irreversible with check stays +1", jev.Answers{Kind: "review", Blast: 1, Check: 0.9}, 1, ""},
		// Rejects: strict > on the up cut points (>= is the contract).
		{"novelty at the cut point steps up", jev.Answers{Kind: "review", Novelty: 0.6, Check: 0.49}, 2, ruleUp},
		// Rejects: strict > on the down cut points.
		{"mechanical and check at the cut points step down", jev.Answers{Kind: "review", Mechanical: 0.7, Check: 0.7}, 0, ruleDown},
		// Rejects: a down step on mechanical alone, without a check.
		{"mechanical without check stays", jev.Answers{Kind: "review", Mechanical: 0.9, Check: 0.6}, 1, ""},
		// Rejects: steps on low signals.
		{"quiet answers keep the kind offset", jev.Answers{Kind: "verify", Novelty: 0.3, Blast: 0.3, Check: 0.2}, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.a.KindConfidence = 0.83
			v := jev.Combine(tc.a, config.DefaultKinds(), jev.DefaultWeights)
			if v.Offset != tc.offset {
				t.Errorf("Offset = %d, want %d (%+v)", v.Offset, tc.offset, v)
			}
			checkVerdict(t, v.Confidence, v.Reason, tc.a.Kind, tc.rule)
		})
	}
}

func checkVerdict(t *testing.T, conf float64, reason, kind, rule string) {
	t.Helper()
	// Rejects: a confidence that is not the kind choice's own.
	if conf != 0.83 {
		t.Errorf("Confidence = %v, want KindConfidence 0.83", conf)
	}
	// Rejects: a reason that hides the kind, its confidence or the rule, or names a rule not applied.
	for _, s := range []string{"jev", "kind " + kind, "c=0.83"} {
		if !strings.Contains(reason, s) {
			t.Errorf("Reason %q lacks %q", reason, s)
		}
	}
	for _, r := range []string{ruleUp, ruleDown} {
		if has := strings.Contains(reason, r); has != (r == rule) {
			t.Errorf("Reason %q: rule %q present = %v, want %v", reason, r, has, !has)
		}
	}
	if strings.Contains(reason, "\n") {
		t.Errorf("Reason %q is multi-line", reason)
	}
}

func TestCombineUpBeatsDown(t *testing.T) {
	// Rejects: up and down both applied (net 0 step), or down winning. With DownCheck 0.3 a check
	// of 0.4 satisfies down (>= 0.3) while still counting as "no check" for up (< 0.5).
	w := jev.DefaultWeights
	w.DownCheck = 0.3
	a := jev.Answers{Kind: "implement", KindConfidence: 0.83, Novelty: 0.9, Mechanical: 0.9, Check: 0.4}
	v := jev.Combine(a, config.DefaultKinds(), w)
	if v.Offset != 1 {
		t.Errorf("Offset = %d, want 1 (implement 0, up wins)", v.Offset)
	}
	checkVerdict(t, v.Confidence, v.Reason, "implement", ruleUp)
}

func TestCombineHonoursWeightsAndKinds(t *testing.T) {
	// Rejects: Combine reading DefaultWeights instead of w.
	w := jev.Weights{UpNovelty: 0.95, UpBlast: 0.95, DownMechanical: 0.95, DownCheck: 0.95}
	a := jev.Answers{Kind: "implement", KindConfidence: 0.83, Novelty: 0.9, Blast: 0.9, Check: 0.1}
	if v := jev.Combine(a, config.DefaultKinds(), w); v.Offset != 0 {
		t.Errorf("raised cut points: Offset = %d, want 0", v.Offset)
	}
	// Rejects: Combine reading config.DefaultKinds instead of the project's table.
	a = jev.Answers{Kind: "implement", KindConfidence: 0.83}
	if v := jev.Combine(a, map[string]int{"implement": 1}, jev.DefaultWeights); v.Offset != 1 {
		t.Errorf("project kinds: Offset = %d, want 1", v.Offset)
	}
}

func TestCombineUnknownKindHasZeroConfidence(t *testing.T) {
	// Ruling: a kind missing from the table (or empty) has no offset to start from, so the verdict
	// carries confidence 0. Explain never relies on it: decode refuses a kind not offered, and the
	// offered "other" falls back to declared before Combine's verdict is used.
	// Rejects: a silent offset 0 reported with Jev's confidence, and a reason hiding the bad kind.
	for _, k := range []string{"refactor", ""} {
		a := jev.Answers{Kind: k, KindConfidence: 0.9}
		v := jev.Combine(a, config.DefaultKinds(), jev.DefaultWeights)
		if v.Confidence != 0 {
			t.Errorf("kind %q: Confidence = %v, want 0", k, v.Confidence)
		}
		if k != "" && !strings.Contains(v.Reason, k) {
			t.Errorf("kind %q: Reason %q does not name it", k, v.Reason)
		}
	}
}

func TestQuestions(t *testing.T) {
	// Rejects: a missing, extra or misnamed question, a wrong answer type, empty instructions,
	// and score questions with the wrong number of levels.
	want := map[string]struct {
		typ    string
		levels int // score levels; 0 when not a score
	}{
		"kind": {"choice", 0}, "depth": {"score", 4}, "ambiguity": {"score", 3}, "blast": {"score", 3},
		"novelty": {"score", 3}, "breadth": {"score", 3}, "check": {"noul", 0}, "mechanical": {"noul", 0},
		"design": {"noul", 0}, "judge": {"noul", 0}, "chain": {"noul", 0},
	}
	qs := jev.Questions(slices.Sorted(maps.Keys(config.DefaultKinds())))
	if got := slices.Sorted(maps.Keys(qs)); !slices.Equal(got, slices.Sorted(maps.Keys(want))) {
		t.Fatalf("question ids = %v", got)
	}
	for id, w := range want {
		q := qs[id]
		if q.Type != w.typ {
			t.Errorf("%s: Type = %q, want %q", id, q.Type, w.typ)
		}
		// Rejects: instructions that do not point Jev at the task in state.
		if !strings.Contains(q.Instructions, "`task`") {
			t.Errorf("%s: Instructions %q do not reference `task`", id, q.Instructions)
		}
		if w.levels > 0 {
			var levels []json.RawMessage
			if err := json.Unmarshal(mustJSON(t, q.Criteria), &levels); err != nil || len(levels) != w.levels {
				t.Errorf("%s: criteria %s, want an array of %d levels", id, mustJSON(t, q.Criteria), w.levels)
			}
		}
	}
	// Rejects: the kind question trusting the declared label (MS: Jev judges the work itself).
	if !strings.Contains(qs["kind"].Instructions, "declared_kind") {
		t.Errorf("kind instructions %q do not tell Jev to ignore `declared_kind`", qs["kind"].Instructions)
	}
}

func TestQuestionsKindCriteriaAreTheKindsPassed(t *testing.T) {
	// covers: R-MOD-065
	// Rejects: a hard-coded kind list (extra kinds Combine cannot place, or a project's kind left
	// out), options re-sorted alphabetically (Jev favours the first option: debug would always lead),
	// and no "other" option for work no kind fits.
	for _, kinds := range [][]string{
		jev.KindOrder(config.DefaultKinds()),
		{"search", "design"},
		{"implement", "refactor"},
	} {
		var opts map[string]json.RawMessage
		raw := mustJSON(t, jev.Questions(kinds)["kind"].Criteria)
		if err := json.Unmarshal(raw, &opts); err != nil {
			t.Fatalf("kinds %v: kind criteria %s is not an object keyed by kind: %v", kinds, raw, err)
		}
		if got, want := objectKeys(t, raw), append(slices.Clone(kinds), jev.Other); !slices.Equal(got, want) {
			t.Errorf("kind criteria options = %v, want %v in that order", got, want)
		}
		// Rejects: a known kind sent without its covers/excludes description.
		for k, v := range opts {
			if k != "refactor" && (string(v) == "null" || string(v) == `""`) {
				t.Errorf("kind %q has no description", k)
			}
		}
	}
}

func TestKindOrder(t *testing.T) {
	// Rejects: alphabetical order (map keys), and project kinds mixed in among the built-ins.
	kinds := config.DefaultKinds()
	kinds["refactor"], kinds["audit"] = 0, 1
	want := []string{"search", "run", "implement", "debug", "research", "review", "verify", "design", "audit", "refactor"}
	if got := jev.KindOrder(kinds); !slices.Equal(got, want) {
		t.Errorf("KindOrder = %v, want %v", got, want)
	}
}

func TestQuestionsWording(t *testing.T) {
	// Rejects: asking Jev about the codebase (it sees only `task`), and compound nouls ("and"/"where"
	// joining two conditions in judge and chain).
	for id, q := range jev.Questions(jev.KindOrder(config.DefaultKinds())) {
		text := q.Instructions + string(mustJSON(t, q.Criteria))
		if strings.Contains(text, "codebase") {
			t.Errorf("%s mentions the codebase: %s", id, text)
		}
		if (id == "judge" || id == "chain") && (strings.Contains(q.Instructions, " and ") || strings.Contains(q.Instructions, " where ")) {
			t.Errorf("%s is compound: %q", id, q.Instructions)
		}
	}
}

// objectKeys is the keys of the JSON object raw in their written order.
func objectKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	var keys []string
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
