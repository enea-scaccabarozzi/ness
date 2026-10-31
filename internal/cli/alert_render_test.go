package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/slack"
	"github.com/enea-scaccabarozzi/ness/internal/atom/telegram"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// alertOf is an alert of the spec's example session.
func alertOf(kind worker.AlertKind, text, page string) worker.Alert {
	return worker.Alert{Kind: kind, Project: "example-lab", Title: "weekly-review-plan",
		Text: text, Page: page, Root: "s-aaaaaaaaaaaa"}
}

// linked is a with the session's Remote Control URL.
func linked(a worker.Alert) worker.Alert {
	a.Session = "https://claude.ai/code/session_01AbC"
	return a
}

// rendered is the alert as the channel's renderer sends it, in one message.
func rendered(t *testing.T, r notify.Renderer, a worker.Alert) string {
	t.Helper()
	parts, err := notify.Split(r, notify.Parse(a.Markdown()))
	if err != nil || len(parts) != 1 {
		t.Fatalf("%d parts, %v", len(parts), err)
	}
	return parts[0].Text()
}

// Rejects an implementation that mislabels a kind, drops the page, leaves the title out of bold or
// unlinked when the session URL is known, leaves a raw URL, glues a list to its paragraph, or lets a
// text with markup characters through unescaped. The golden files are written by hand from the
// spec's example, never from the renderer's output.
func TestAlertRendersPerKindAndChannel(t *testing.T) {
	// covers: R-WRK-082
	cases := []struct {
		name string
		a    worker.Alert
	}{
		{"needs-you", alertOf(worker.AlertNeedsYou, "Go on L0b? Band study over the three income cells, 2 children.", "")},
		{"checkpoint", alertOf(worker.AlertCheckpoint, "Rung two: re-score the cells.", "https://claude.ai/x")},
		{"trouble", alertOf(worker.AlertTrouble, "is blocked on a prompt in its tab", "")},
		{"your-move", alertOf(worker.AlertYourMove, "idle for 30 minutes; nothing appears to be running", "")},
		{"needs-you-escaped", alertOf(worker.AlertNeedsYou, "Is a<b & c>d ok?", "")},
		{"needs-you-links", linked(alertOf(worker.AlertNeedsYou,
			"Which band?\n- low, see https://example.com/bands?q=1.\n- high, as in https://claude.ai/artifact/B2",
			"https://claude.ai/artifact/P1"))},
		{"checkpoint-links", linked(alertOf(worker.AlertCheckpoint, "Rung two: re-score the cells.", "https://claude.ai/x"))},
		{"trouble-links", linked(alertOf(worker.AlertTrouble, "is blocked on a prompt in its tab", ""))},
		{"your-move-links", linked(alertOf(worker.AlertYourMove, "idle for 30 minutes; nothing appears to be running", ""))},
	}
	channels := map[string]notify.Renderer{"telegram": telegram.Channel{}, "slack": slack.Channel{}}
	for _, c := range cases {
		for name, r := range channels {
			t.Run(c.name+"/"+name, func(t *testing.T) {
				testkit.Golden(t, filepath.Join("testdata", "golden", "alert_"+c.name+"_"+name+".txt"), rendered(t, r, c.a))
			})
		}
	}
}

// Rejects an implementation that lets a full-length text split into two messages.
func TestAlertWithTheLongestTextIsOneMessage(t *testing.T) {
	// covers: R-WRK-082
	longest := strings.Repeat("é", 400) + "…" // the 400-rune cap and the cut mark
	a := alertOf(worker.AlertNeedsYou, longest, "https://claude.ai/"+strings.Repeat("p", 150))
	for _, r := range []notify.Renderer{telegram.Channel{}, slack.Channel{}} {
		if got := rendered(t, r, a); !strings.Contains(got, longest) {
			t.Fatalf("%T: the text was altered: %q", r, got)
		}
	}
}
