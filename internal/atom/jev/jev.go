package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// maxTask is the head of the task sent, in bytes (cut on a rune boundary): under Jev's 32k-token
// per-question limit for any script.
const maxTask = 60_000

// backoff is the wait before the one retry of a 429 or 529 (the hook is waiting).
const backoff = 500 * time.Millisecond

// Policy is the jev selector: one /v1/systemone request, combined by Combine, falling back to
// Fallback on any failure or a confidence below Threshold.
type Policy struct {
	Endpoint, Model, Key string
	Timeout              time.Duration
	Client               *http.Client
	Kinds                map[string]int
	Weights              Weights
	Threshold            float64
	Fallback             session.Policy
}

// Choose answers Jev's verdict, or the fallback's with the cause in its reason.
func (p Policy) Choose(b session.Brief) (session.Verdict, error) {
	_, v, err := p.Explain(b)
	return v, err
}

// Explain is Choose that also returns the decoded answers (R-MOD-065..068). After a fallback the
// answers carry Fallback; they are kept when Jev answered (low confidence, kind other).
// With no declared kind the fallback is base, not ErrNoKind (specs.md §2).
func (p Policy) Explain(b session.Brief) (Answers, session.Verdict, error) {
	a, err := p.ask(b)
	if err == nil {
		v := Combine(a, p.Kinds, p.Weights)
		if _, known := p.Kinds[a.Kind]; !known {
			err = fmt.Errorf(": kind %s", a.Kind)
		} else if v.Confidence < p.Threshold {
			err = fmt.Errorf(": confidence %.2f < %.2f", v.Confidence, p.Threshold)
		} else {
			return a, v, nil
		}
	} else {
		a = Answers{}
	}
	a.Fallback = true
	cause := err.Error()
	if p.Key != "" {
		cause = strings.ReplaceAll(cause, p.Key, "***")
	}
	v, ferr := p.Fallback.Choose(b)
	if errors.Is(ferr, session.ErrNoKind) {
		v, ferr = session.Verdict{Confidence: 1, Reason: "no kind: base"}, nil
	}
	v.Reason = "jev" + cause + "; " + v.Reason
	return a, v, ferr
}

// answer is one entry of the response's answers, any of the three types.
type answer struct {
	Noul          *float64
	Choice        string
	Score         *float64
	Confidence    float64
	Probabilities map[string]float64
}

// ask sends the request and decodes the answers; its errors read as ": <cause>" and never hold the key.
func (p Policy) ask(b session.Brief) (Answers, error) {
	if p.Key == "" {
		return Answers{}, errors.New(": no key (models.jev.key)")
	}
	qs := Questions(KindOrder(p.Kinds))
	body, err := json.Marshal(map[string]any{"model": p.Model, "state": state(b), "questions": wire(qs)})
	if err != nil {
		return Answers{}, fmt.Errorf(": encode: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	raw, err := p.post(ctx, body)
	if err != nil {
		return Answers{}, err
	}
	var resp struct{ Answers map[string]answer }
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Answers{}, fmt.Errorf(": malformed body: %w", err)
	}
	return decode(resp.Answers, qs)
}

// post sends body, retrying once on 429 or 529, and returns the 200 response body.
func (p Policy) post(ctx context.Context, body []byte) ([]byte, error) {
	for try := 0; ; try++ {
		status, raw, err := p.send(ctx, body)
		switch {
		case err != nil:
			return nil, err
		case status == http.StatusOK:
			return raw, nil
		case try > 0 || (status != http.StatusTooManyRequests && status != 529):
			return nil, fmt.Errorf(": %d", status) // the body may echo the key: never quoted
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(": timeout after %v", p.Timeout)
		case <-time.After(backoff):
		}
	}
}

// send makes one request and returns its status and body.
func (p Policy) send(ctx context.Context, body []byte) (status int, raw []byte, err error) {
	url := strings.TrimSuffix(p.Endpoint, "/") + "/v1/systemone"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, errors.New(": bad endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, fmt.Errorf(": timeout after %v", p.Timeout)
		}
		return 0, nil, errors.New(": request failed")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // read fully below
	raw, err = io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, errors.New(": reading the body failed")
	}
	return resp.StatusCode, raw, nil
}

// state is what Jev judges: the task (head kept when long) and the brief's labels, empty ones omitted.
func state(b session.Brief) map[string]string {
	task := b.Text
	if len(task) > maxTask {
		cut := maxTask
		for cut > 0 && !utf8.RuneStart(task[cut]) {
			cut--
		}
		task = task[:cut] + "\n[... truncated]"
	}
	s := map[string]string{"task": task}
	for k, v := range map[string]string{"role": b.Role, "agent_type": b.AgentType, "declared_kind": b.Kind} {
		if v != "" {
			s[k] = v
		}
	}
	return s
}

// wire is the question set in the API's lower-case field names.
func wire(qs map[string]Question) map[string]map[string]any {
	out := make(map[string]map[string]any, len(qs))
	for id, q := range qs {
		w := map[string]any{"type": q.Type, "instructions": q.Instructions}
		if q.Criteria != nil {
			w["criteria"] = q.Criteria
		}
		out[id] = w
	}
	return out
}

// decode checks every question has a well-formed answer and normalises the scores to 0..1.
func decode(got map[string]answer, qs map[string]Question) (Answers, error) {
	var a Answers
	vals := map[string]*float64{
		"kind": &a.KindConfidence, "depth": &a.Depth, "ambiguity": &a.Ambiguity, "blast": &a.Blast,
		"novelty": &a.Novelty, "breadth": &a.Breadth, "check": &a.Check, "mechanical": &a.Mechanical,
		"design": &a.Design, "judge": &a.Judge, "chain": &a.Chain,
	}
	for _, id := range slices.Sorted(maps.Keys(qs)) {
		r, ok := got[id]
		if !ok {
			return Answers{}, fmt.Errorf(": answer %s missing", id)
		}
		v, ok := value(qs[id], r)
		if !ok {
			return Answers{}, fmt.Errorf(": answer %s malformed", id)
		}
		*vals[id] = v
		if qs[id].Type == "choice" {
			a.Kind = r.Choice
		}
	}
	return a, nil
}

// value is an answer's number: a choice's confidence, a score over its top level, a noul's
// probability. An answer out of range, or a choice not offered, is malformed (R-MOD-068).
func value(q Question, r answer) (float64, bool) {
	switch q.Type {
	case "choice":
		opts, ok := q.Criteria.(Options)
		return r.Confidence, ok && opts.has(r.Choice) && unit(r.Confidence) && sumsToOne(r.Probabilities)
	case "score":
		return scoreValue(q, r)
	default:
		if r.Noul == nil {
			return 0, false
		}
		return *r.Noul, unit(*r.Noul)
	}
}

// scoreValue is a score over its top level; false when out of the levels or malformed.
func scoreValue(q Question, r answer) (float64, bool) {
	levels, ok := q.Criteria.([]string)
	if !ok || r.Score == nil || len(levels) < 2 || !sumsToOne(r.Probabilities) || !unit(r.Confidence) {
		return 0, false
	}
	v := *r.Score / float64(len(levels)-1)
	return v, unit(v)
}

// unit reports x within 0..1.
func unit(x float64) bool { return x >= 0 && x <= 1 }

// sumsToOne reports a probability map summing to 1 within rounding.
func sumsToOne(ps map[string]float64) bool {
	s := 0.0
	for _, p := range ps {
		s += p
	}
	return math.Abs(s-1) < 0.05
}
