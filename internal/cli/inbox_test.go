package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestInboxLineGolden(t *testing.T) {
	// covers: R-MSG-030, R-MSG-031, R-MSG-043, R-MSG-045
	now := time.Date(2026, 10, 8, 13, 10, 0, 0, time.UTC)
	cases := []struct {
		it   store.Item
		want string
	}{
		{
			store.Item{ID: "m-abcdefgh", Sender: "s-bbbbbbbbbbbb", Intent: "request", Body: "need a decision", CreatedAt: now.Add(-3*time.Hour - 10*time.Minute)},
			"m-abcdefgh\ts-bbbbbbbbbbbb\trequest\t3h\tneed a decision",
		},
		{
			store.Item{ID: "m-abcdefgh", Sender: store.Operator, Intent: "reply", Body: "yes\nmerge\tit", CreatedAt: now.Add(-59 * time.Second)},
			"m-abcdefgh\toperator\treply\t0m\tyes merge it",
		},
		{
			store.Item{ID: "m-abcdefgh", Sender: "s-bbbbbbbbbbbb", Intent: "handoff", Disposition: store.Deferred, Body: strings.Repeat("x", 100), CreatedAt: now.Add(-50 * time.Hour)},
			"m-abcdefgh\ts-bbbbbbbbbbbb\thandoff deferred\t2d\t" + strings.Repeat("x", 80),
		},
	}
	for _, c := range cases {
		if got := inboxLine(c.it, now); got != c.want {
			t.Errorf("inboxLine = %q\nwant        %q", got, c.want)
		}
	}
}
