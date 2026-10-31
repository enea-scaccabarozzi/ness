package jev_test

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/atom/jev"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

func TestRetryThenSuccess(t *testing.T) {
	// covers: R-MOD-066
	// Rejects: giving up on the first 429, or a retry that resends a different request.
	synctest.Test(t, func(t *testing.T) {
		f := &fakeJev{replies: []reply{{status: 429, body: `{}`}, {status: 200, body: okBody(t, fullAnswers())}}}
		v, err := choose(t, newPolicy(f), brief())
		if want := jev.Combine(wantAnswers, config.DefaultKinds(), jev.DefaultWeights); err != nil || v != want {
			t.Errorf("Choose = %+v, %v; want %+v", v, err, want)
		}
		if f.count() != 2 || string(f.bodies[0]) != string(f.bodies[1]) {
			t.Errorf("%d requests, want 2 identical ones", f.count())
		}
	})
}

func TestTimeout(t *testing.T) {
	// covers: R-MOD-066
	// Rejects: ignoring p.Timeout (the fake hangs forever: synctest reports the deadlock), retrying
	// a timeout, and a timeout surfaced as an error instead of the declared fallback.
	synctest.Test(t, func(t *testing.T) {
		f := &fakeJev{replies: []reply{{hang: true}}}
		p := newPolicy(f)
		start := time.Now()
		v, err := choose(t, p, brief())
		wantFallback(t, v, err, "timeout", "Timeout", "deadline")
		if d := time.Since(start); f.count() != 1 || d < p.Timeout || d > p.Timeout+time.Second {
			t.Errorf("%d requests over %v, want 1 cut at %v", f.count(), d, p.Timeout)
		}
	})
}

func TestNoKeySendsNothing(t *testing.T) {
	// covers: R-MOD-067
	// Rejects: an unauthenticated request ("Bearer "), and a fallback that does not say why.
	f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
	p := newPolicy(f)
	p.Key = ""
	v, err := choose(t, p, brief())
	wantFallback(t, v, err, "no key (models.jev.key)")
	if f.count() != 0 {
		t.Errorf("%d requests without a key, want 0", f.count())
	}
}

func TestFallbackWithNoKindIsBase(t *testing.T) {
	// covers: R-MOD-067
	// Ruling (specs.md §2): Jev down and no declared kind recommends base, so the same prompt is
	// judged, never refused, whatever the network does.
	// Rejects: passing ErrNoKind through (the hook refuses), a low confidence Pick turns into a
	// "low confidence" reason, or a reason without the cause.
	f := &fakeJev{replies: []reply{{status: 401, body: `{}`}}}
	v, err := choose(t, newPolicy(f), session.Brief{Text: "do something", Role: "subagent"})
	if err != nil || v != (session.Verdict{Offset: 0, Confidence: 1, Reason: "jev: 401; no kind: base"}) {
		t.Errorf("Choose = %+v, %v; want base with confidence 1 and reason \"jev: 401; no kind: base\"", v, err)
	}
}

func TestLowConfidenceKeepsAnswers(t *testing.T) {
	// covers: R-MOD-067
	// Rejects: dropping the answers on the confidence gate (calibration needs exactly those), not
	// marking the fallback, and marking a fallback when Jev's verdict was used.
	f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
	p := newPolicy(f)
	p.Threshold = 0.9
	a, v, err := p.Explain(brief())
	wantFallback(t, v, err, "confidence 0.83 < 0.90")
	want := wantAnswers
	want.Fallback = true
	if !answersClose(a, want) || !a.Fallback {
		t.Errorf("answers %+v, want %+v with Fallback", a, want)
	}
	p.Threshold = 0.5
	if a, _, _ := p.Explain(brief()); a.Fallback {
		t.Errorf("answers %+v marked as a fallback above the threshold", a)
	}
	f.replies = []reply{{status: 500}}
	if a, _, _ := p.Explain(brief()); !a.Fallback || a.Kind != "" {
		t.Errorf("after a 500: answers %+v, want none with Fallback", a)
	}
}

func TestEndpointTrailingSlash(t *testing.T) {
	// covers: R-MOD-065
	// Rejects: posting to "//v1/systemone" when the endpoint ends in "/" (a silent fallback every pick).
	f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
	p := newPolicy(f)
	p.Endpoint = "http://jev.test/"
	_, _ = choose(t, p, brief())
	if f.count() != 1 || f.reqs[0].URL.String() != "http://jev.test/v1/systemone" {
		t.Errorf("requests %d, URL %v", f.count(), f.reqs)
	}
}

func TestLongMultibyteTaskCutOnARune(t *testing.T) {
	// covers: R-MOD-065
	// Rejects: a cut by rune count (60,000 two-byte runes is 120,000 bytes, over the token budget
	// for dense scripts) and a raw byte cut that splits a rune (invalid UTF-8 in the request).
	text := "a" + strings.Repeat("é", 40_000) // é starts at odd offsets, so byte 60,000 is mid-rune
	b := brief()
	b.Text = text
	f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
	_, _ = choose(t, newPolicy(f), b)
	if !utf8.Valid(f.bodies[0]) {
		t.Fatal("request body is not valid UTF-8")
	}
	var body struct{ State struct{ Task string } }
	if err := json.Unmarshal(f.bodies[0], &body); err != nil {
		t.Fatal(err)
	}
	if want := text[:59_999] + "\n[... truncated]"; body.State.Task != want {
		t.Errorf("task sent has %d bytes, want the 59,999-byte head plus the marker", len(body.State.Task))
	}
}

func TestLongTaskTruncated(t *testing.T) {
	// covers: R-MOD-065
	// Rejects: sending 200k characters (over Jev's 32k-token question limit), keeping the tail
	// instead of the head, and cutting silently with no marker.
	head := strings.Repeat("a", 60_000)
	b := brief()
	b.Text = head + strings.Repeat("§", 140_000)
	f := &fakeJev{replies: []reply{{status: 200, body: okBody(t, fullAnswers())}}}
	_, _ = choose(t, newPolicy(f), b)
	if f.count() != 1 {
		t.Fatalf("%d requests, want 1", f.count())
	}
	var body struct{ State struct{ Task string } }
	if err := json.Unmarshal(f.bodies[0], &body); err != nil {
		t.Fatal(err)
	}
	task := body.State.Task
	if !strings.HasPrefix(task, head) || strings.Contains(task, "§") || len(task) <= len(head) || len(task) > len(head)+200 {
		t.Errorf("task sent has %d bytes (head kept %v, tail present %v), want the 60,000-char head plus a marker",
			len(task), strings.HasPrefix(task, head), strings.Contains(task, "§"))
	}
}
