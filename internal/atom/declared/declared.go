// Package declared is the selector that reads the work kind the caller declares and answers its offset from base.
package declared

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// NamedKinds is the work kind of each named ness subagent, a fresh map per call.
func NamedKinds() map[string]string {
	return map[string]string{
		"ness-sweep": "search", "ness-review": "review", "ness-verify": "verify", "ness-design": "design",
	}
}

// Policy answers the project's offset for the declared kind (R-MOD-008).
type Policy struct{ Kinds map[string]int }

// Choose takes the kind from b.Kind, else from the named agent type; kinds match exactly.
func (p Policy) Choose(b session.Brief) (session.Verdict, error) {
	k := b.Kind
	if k == "" {
		k = NamedKinds()[b.AgentType]
	}
	off, ok := p.Kinds[k]
	if !ok {
		return session.Verdict{}, fmt.Errorf("%w: one of %s", session.ErrNoKind,
			strings.Join(slices.Sorted(maps.Keys(p.Kinds)), ", "))
	}
	return session.Verdict{Offset: off, Confidence: 1, Reason: "kind " + k}, nil
}
