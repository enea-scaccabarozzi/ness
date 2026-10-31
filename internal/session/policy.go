package session

import (
	"errors"
	"fmt"

	"github.com/enea-scaccabarozzi/ness/internal/ladder"
)

// ErrNoKind is returned by a selector that needs a work kind and has none it knows.
var ErrNoKind = errors.New("name the work kind")

// Brief is the selector's input (R-MOD-041): the task text, the role (root, child or subagent),
// the client, the declared work kind and the subagent type when there is one.
type Brief struct {
	Text, Role, Client, Kind, AgentType string
}

// Verdict is a selector's answer: an offset from the base rung, a confidence in 0..1 and a
// one-line reason.
type Verdict struct {
	Offset     int
	Confidence float64
	Reason     string
}

// Policy is the selector seam (design §2, D17): declared, command, a router later.
type Policy interface {
	Choose(Brief) (Verdict, error)
}

// Choice is the rung a launch gets, with the confidence and the one-line reason recorded for
// stats (R-MOD-043).
type Choice struct {
	Tier, Model, Effort string
	Index               int
	Confidence          float64
	Reason              string
}

// Pick turns the selector's verdict into a rung of l; it is the only place that falls back to
// base: on a selector error (returned with the base choice) and below threshold.
func Pick(p Policy, b Brief, l ladder.Ladder, threshold float64) (Choice, error) {
	v, err := p.Choose(b)
	if err != nil {
		return choice(l, 0, 0, "base: "+err.Error()), err
	}
	if v.Confidence < threshold {
		return choice(l, 0, v.Confidence, fmt.Sprintf("base: low confidence (%.2f): %s", v.Confidence, v.Reason)), nil
	}
	reason := v.Reason
	if _, ok := l.At(v.Offset); !ok {
		reason += " (clamped)"
	}
	return choice(l, v.Offset, v.Confidence, reason), nil
}

func choice(l ladder.Ladder, offset int, confidence float64, reason string) Choice {
	r, _ := l.At(offset)
	i, _ := l.Index(r.Name)
	return Choice{Tier: r.Name, Model: r.Model, Effort: r.Effort, Index: i, Confidence: confidence, Reason: reason}
}
