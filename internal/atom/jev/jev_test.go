package jev_test

import (
	"encoding/json"
	"io"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/declared"
	"github.com/enea-scaccabarozzi/ness/internal/atom/jev"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// fakeKey is distinctive so a leak into any error or reason is unmistakable.
const fakeKey = "tsk-FAKE-c0ffee-0123456789-NEVER-PRINT"

// reply is one canned endpoint answer; hang blocks until the request is cancelled.
type reply struct {
	status int
	body   string
	hang   bool
}

// fakeJev is a hand-written /v1/systemone endpoint served in memory (no network, so synctest's
// fake clock drives the retry backoff and the timeout). Replies are served in order; the last one
// repeats.
type fakeJev struct {
	mu      sync.Mutex
	replies []reply
	reqs    []*http.Request
	bodies  [][]byte
}

func (f *fakeJev) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs, f.bodies = append(f.reqs, r), append(f.bodies, body)
	rp := f.replies[min(len(f.reqs), len(f.replies))-1]
	f.mu.Unlock()
	if rp.hang {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(rp.status)
	_, _ = rec.WriteString(rp.body)
	return rec.Result(), nil
}

func (f *fakeJev) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func newPolicy(f *fakeJev) jev.Policy {
	return jev.Policy{
		Endpoint: "http://jev.test", Model: "jev-1.13.0", Key: fakeKey, Timeout: 5 * time.Second,
		Client: &http.Client{Transport: f}, Kinds: config.DefaultKinds(), Weights: jev.DefaultWeights,
		Threshold: 0.5, Fallback: declared.Policy{Kinds: config.DefaultKinds()},
	}
}

func brief() session.Brief {
	return session.Brief{Text: "Rename cfg to config in internal/launch; go test ./internal/launch passes.",
		Role: "subagent", Client: "claude", Kind: "implement", AgentType: "general-purpose"}
}

// score is a score answer whose score is the probability-weighted level.
func score(conf float64, probs ...float64) map[string]any {
	s, m := 0.0, map[string]float64{}
	for i, p := range probs {
		s += float64(i) * p
		m[string(rune('0'+i))] = p
	}
	return map[string]any{"type": "score", "score": s, "confidence": conf, "probabilities": m}
}

func noul(p float64) map[string]any { return map[string]any{"type": "noul", "noul": p} }

// fullAnswers is a complete answer set; every field has a distinct value so a swapped field shows.
func fullAnswers() map[string]any {
	return map[string]any{
		"kind": map[string]any{"type": "choice", "choice": "implement", "confidence": 0.83,
			"probabilities": map[string]float64{"implement": 0.9, "search": 0.05, "review": 0.05}},
		"depth":     score(0.6, 0.1, 0.4, 0.4, 0.1), // 1.5 of 3
		"ambiguity": score(0.7, 0.8, 0.2, 0),        // 0.2 of 2
		"blast":     score(0.6, 0.7, 0.3, 0),        // 0.3 of 2
		"novelty":   score(0.8, 0.05, 0.1, 0.85),    // 1.8 of 2
		"breadth":   score(0.5, 0.1, 0.5, 0.4),      // 1.3 of 2
		"check":     noul(0.1), "mechanical": noul(0.2), "design": noul(0.3), "judge": noul(0.4), "chain": noul(0.6),
	}
}

// wantAnswers is fullAnswers decoded: scores divided by their top level.
var wantAnswers = jev.Answers{Kind: "implement", KindConfidence: 0.83, Depth: 0.5, Ambiguity: 0.1,
	Blast: 0.15, Novelty: 0.9, Breadth: 0.65, Check: 0.1, Mechanical: 0.2, Design: 0.3, Judge: 0.4, Chain: 0.6}

func okBody(t *testing.T, answers map[string]any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"model": "jev-1.13.0", "answers": answers,
		"usage": map[string]int{"input_tokens": 900, "output_tokens": 11}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// choose runs Choose and fails on any key leak in the verdict or the error.
func choose(t *testing.T, p jev.Policy, b session.Brief) (session.Verdict, error) {
	t.Helper()
	v, err := p.Choose(b)
	// Rejects: an error or reason that quotes the key, the Authorization header or a body echoing it.
	if strings.Contains(v.Reason, fakeKey) || (err != nil && strings.Contains(err.Error(), fakeKey)) {
		t.Fatalf("key leaked: reason %q, err %v", v.Reason, err)
	}
	return v, err
}

// wantFallback checks the declared verdict for "implement" (0) came back, reason "jev <cause>; kind implement".
func wantFallback(t *testing.T, v session.Verdict, err error, causes ...string) {
	t.Helper()
	if err != nil {
		t.Fatalf("fallback returned error %v", err)
	}
	if v.Offset != 0 || v.Confidence != 1 {
		t.Errorf("verdict %+v, want the declared one (offset 0, confidence 1)", v)
	}
	if !strings.HasPrefix(v.Reason, "jev") || !strings.HasSuffix(v.Reason, "; kind implement") {
		t.Errorf("reason %q, want \"jev <cause>; kind implement\"", v.Reason)
	}
	if len(causes) > 0 && !slices.ContainsFunc(causes, func(c string) bool { return strings.Contains(v.Reason, c) }) {
		t.Errorf("reason %q names none of the causes %q", v.Reason, causes)
	}
}

func TestRequest(t *testing.T) {
	// covers: R-MOD-065
	// Rejects: a GET or a wrong path, a missing/raw Authorization header, a hard-coded model instead
	// of p.Model, state fields renamed or sent empty, the client leaking into state, a question set
	// that is not exactly Questions(kinds), and Go-cased question fields (Type instead of type).
	for name, b := range map[string]session.Brief{
		"all fields": brief(),
		"task only":  {Text: "map the packages", Client: "codex"},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
			_, _ = choose(t, newPolicy(f), b)
			if f.count() != 1 {
				t.Fatalf("%d requests, want 1", f.count())
			}
			r := f.reqs[0]
			if r.Method != http.MethodPost || r.URL.String() != "http://jev.test/v1/systemone" {
				t.Errorf("request %s %s", r.Method, r.URL)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer "+fakeKey {
				t.Errorf("Authorization = %q", got)
			}
			if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q", ct)
			}
			var body struct {
				Model     string
				State     map[string]string
				Questions map[string]map[string]json.RawMessage
			}
			if err := json.Unmarshal(f.bodies[0], &body); err != nil {
				t.Fatalf("body %s: %v", f.bodies[0], err)
			}
			if body.Model != "jev-1.13.0" {
				t.Errorf("model = %q, want p.Model", body.Model)
			}
			want := map[string]string{"task": b.Text}
			for k, v := range map[string]string{"role": b.Role, "agent_type": b.AgentType, "declared_kind": b.Kind} {
				if v != "" {
					want[k] = v
				}
			}
			if !maps.Equal(body.State, want) {
				t.Errorf("state = %v, want %v", body.State, want)
			}
			checkQuestions(t, body.Questions)
		})
	}
}

func checkQuestions(t *testing.T, got map[string]map[string]json.RawMessage) {
	t.Helper()
	want := jev.Questions(slices.Sorted(maps.Keys(config.DefaultKinds())))
	if !slices.Equal(slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want))) {
		t.Fatalf("question ids = %v", slices.Sorted(maps.Keys(got)))
	}
	for id, q := range want {
		wantCrit := mustJSON(t, q.Criteria)
		if c, ok := got[id]["criteria"]; (ok || q.Criteria != nil) && !jsonEqual(c, wantCrit) {
			t.Errorf("%s: criteria %s, want %s", id, c, wantCrit)
		}
		if !jsonEqual(got[id]["type"], mustJSON(t, q.Type)) || !jsonEqual(got[id]["instructions"], mustJSON(t, q.Instructions)) {
			t.Errorf("%s: sent %v, want type %q and its instructions", id, got[id], q.Type)
		}
	}
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

func TestSuccessIsCombine(t *testing.T) {
	// Rejects: ignoring the answers (returning the declared verdict), mis-normalised scores (not
	// divided by the top level), swapped fields, Combine fed DefaultKinds instead of p.Kinds, and
	// calling the fallback first so a brief with no declared kind fails.
	kinds := config.DefaultKinds()
	kinds["implement"] = -2
	for _, b := range []session.Brief{brief(), {Text: brief().Text, Role: "subagent"}} {
		f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
		p := newPolicy(f)
		p.Kinds = kinds
		a, v, err := p.Explain(b)
		if err != nil {
			t.Fatalf("Kind %q: %v", b.Kind, err)
		}
		if !answersClose(a, wantAnswers) {
			t.Errorf("answers %+v, want %+v", a, wantAnswers)
		}
		if want := jev.Combine(wantAnswers, kinds, jev.DefaultWeights); v != want {
			t.Errorf("Explain verdict %+v, want Combine's %+v", v, want)
		}
		if cv, err := choose(t, p, b); err != nil || cv != v {
			t.Errorf("Choose = %+v, %v; want Explain's %+v", cv, err, v)
		}
	}
}

func answersClose(a, b jev.Answers) bool {
	x := []float64{a.KindConfidence, a.Depth, a.Ambiguity, a.Blast, a.Novelty, a.Breadth, a.Check, a.Mechanical, a.Design, a.Judge, a.Chain}
	y := []float64{b.KindConfidence, b.Depth, b.Ambiguity, b.Blast, b.Novelty, b.Breadth, b.Check, b.Mechanical, b.Design, b.Judge, b.Chain}
	return a.Kind == b.Kind && slices.EqualFunc(x, y, func(p, q float64) bool { return math.Abs(p-q) < 1e-9 })
}

func TestFallbacks(t *testing.T) {
	// covers: R-MOD-066, R-MOD-067, R-MOD-068
	// Each row rejects treating that response as a success, panicking on it, or retrying what is
	// not retryable; requests is the exact number the endpoint must see.
	edit := func(f func(map[string]any)) string {
		a := fullAnswers()
		f(a)
		return okBody(t, a)
	}
	cases := []struct {
		name     string
		replies  []reply
		requests int
		causes   []string
	}{
		// Rejects: a confidence gate that ignores Threshold (0.30 < 0.5).
		{"low confidence", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["kind"].(map[string]any)["confidence"] = 0.3
		})}}, 1, nil},
		// Rejects: retrying a 401, or a reason without the status.
		{"401", []reply{{status: 401, body: `{"detail":{"message":"bad key ` + fakeKey + `"}}`}}, 1, []string{"401"}},
		// Rejects: no retry on 529, or a second retry (the hook is waiting).
		{"529 twice", []reply{{status: 529, body: `{}`}, {status: 529, body: `{}`}, {status: 200, body: okBody(t, fullAnswers())}}, 2, []string{"529"}},
		{"500", []reply{{status: 500, body: `oops`}}, 1, []string{"500"}},
		{"malformed body", []reply{{status: 200, body: `{"answers": {"kind": `}}, 1, nil},
		// Rejects: a missing kind read as "" or a panic on a nil map entry.
		{"kind missing", []reply{{status: 200, body: edit(func(a map[string]any) { delete(a, "kind") })}}, 1, []string{"kind"}},
		// Rejects: a missing noul silently read as 0 (it would look like "no check").
		{"check missing", []reply{{status: 200, body: edit(func(a map[string]any) { delete(a, "check") })}}, 1, []string{"check"}},
		{"kind probabilities sum 1.4", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["kind"].(map[string]any)["probabilities"] = map[string]float64{"implement": 0.9, "search": 0.5}
		})}}, 1, nil},
		// Rejects: accepting any non-empty choice (M1: "refactor" was combined to base, not the fallback).
		{"choice not offered", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["kind"].(map[string]any)["choice"] = "refactor"
		})}}, 1, []string{"answer kind malformed"}},
		// Rejects: "other" combined as an unknown kind to base instead of the declared fallback.
		{"choice other", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["kind"].(map[string]any)["choice"] = "other"
		})}}, 1, []string{"kind other"}},
		// Rejects: a kind confidence above 1 clearing any threshold.
		{"kind confidence 1.7", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["kind"].(map[string]any)["confidence"] = 1.7
		})}}, 1, []string{"answer kind malformed"}},
		// Rejects: a noul read as a probability without a range check.
		{"noul 7.5", []reply{{status: 200, body: edit(func(a map[string]any) { a["check"] = noul(7.5) })}}, 1,
			[]string{"answer check malformed"}},
		{"noul -0.1", []reply{{status: 200, body: edit(func(a map[string]any) { a["design"] = noul(-0.1) })}}, 1,
			[]string{"answer design malformed"}},
		// Rejects: a score index past the last level (5 of 0..3) normalised above 1.
		{"score past the levels", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["depth"].(map[string]any)["score"] = 5
		})}}, 1, []string{"answer depth malformed"}},
		{"score confidence -0.2", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["blast"].(map[string]any)["confidence"] = -0.2
		})}}, 1, []string{"answer blast malformed"}},
		// Rejects: checking the sum on the kind choice only.
		{"depth probabilities sum 0.5", []reply{{status: 200, body: edit(func(a map[string]any) {
			a["depth"] = score(0.6, 0.1, 0.2, 0.1, 0.1)
		})}}, 1, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := &fakeJev{replies: tc.replies}
				start := time.Now()
				v, err := choose(t, newPolicy(f), brief())
				wantFallback(t, v, err, tc.causes...)
				if f.count() != tc.requests {
					t.Errorf("%d requests, want %d", f.count(), tc.requests)
				}
				// Rejects: a backoff above 1s (fake clock; responses take no time).
				if d := time.Since(start); d > time.Second {
					t.Errorf("took %v, want at most one backoff of <= 1s", d)
				}
			})
		})
	}
}
