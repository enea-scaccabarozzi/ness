package jev

import (
	"fmt"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Weights are the cut points Combine applies to the answers.
type Weights struct{ UpNovelty, UpBlast, DownMechanical, DownCheck float64 }

// DefaultWeights are the calibrated cut points (read-only; calibration edits the literal).
var DefaultWeights = Weights{UpNovelty: 0.6, UpBlast: 0.6, DownMechanical: 0.7, DownCheck: 0.7}

// Combine turns the answers into a verdict: the kind's offset moved at most one step.
func Combine(a Answers, kinds map[string]int, w Weights) session.Verdict {
	off, ok := kinds[a.Kind]
	if !ok {
		return session.Verdict{Reason: fmt.Sprintf("jev: unknown kind %q", a.Kind)}
	}
	reason := fmt.Sprintf("jev: kind %s (c=%.2f)", a.Kind, a.KindConfidence)
	switch {
	case stepsUp(a, w):
		off++
		reason += " +1 novel/irreversible without a check"
	case stepsDown(a, w):
		off--
		reason += " -1 mechanical with a check"
	}
	return session.Verdict{Offset: off, Confidence: a.KindConfidence, Reason: reason}
}

// stepsUp reports novel, design-choice or irreversible work without a check; design work already sits high.
func stepsUp(a Answers, w Weights) bool {
	risky := a.Novelty >= w.UpNovelty || a.Design >= w.UpNovelty || a.Blast >= w.UpBlast
	return risky && a.Check < 0.5 && a.Kind != "design"
}

// stepsDown reports mechanical work with a check; search and run already sit at the floor.
func stepsDown(a Answers, w Weights) bool {
	return a.Mechanical >= w.DownMechanical && a.Check >= w.DownCheck && a.Kind != "search" && a.Kind != "run"
}
