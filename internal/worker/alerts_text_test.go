package worker_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// alertCap is the longest alert text, the "…" of a cut aside (R-WRK-082).
const alertCap = 400

// words normalises s to its words joined by single spaces: the tests do not pin how lines join.
func words(s string) string { return strings.Join(strings.Fields(s), " ") }

// alertOfItem sends one operator item from a root and returns the one alert it raised.
func alertOfItem(t *testing.T, intent, body string) worker.Alert {
	t.Helper()
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.send(sA, store.Operator, intent, body, false)
	b.pass()
	if len(a.got) != 1 {
		t.Fatalf("alerts = %+v, want one", a.got)
	}
	return a.got[0]
}

// Rejects the field's one-line Ask ("Ship it?" with the question's terms lost below it): the
// phone gets every line of the Ask section, and nothing of the section after it.
func TestAskAlertCarriesTheWholeAskSection(t *testing.T) {
	// covers: R-WRK-080
	body := "## Done\nd\n## Now\nn\n## Next\nx\n## Ask\nShip it?\nWith the caveats in the page.\n\n" +
		"Or hold for the docs.\n## Lessons\nnot this\n"
	al := alertOfItem(t, store.IntentCheckpoint, body)
	want := "Ship it? With the caveats in the page. Or hold for the docs."
	if al.Kind != worker.AlertNeedsYou || words(al.Text) != want {
		t.Fatalf("alert = %+v, want Needs you with %q", al, want)
	}
}

// Rejects a Next text cut at its first line (a wrapped item lost its end) or one that runs on
// into the next item, the next paragraph or the Page line.
func TestCheckpointAlertCarriesTheFirstNextItem(t *testing.T) {
	// covers: R-WRK-080
	for name, c := range map[string]struct{ next, want, not string }{
		"a list item with its continuation": {
			"- rung two: score the cells\n  then write the table\n- rung three\n", "then write the table", "rung three"},
		"a paragraph up to the blank line": {
			"rung two: score the cells\nthen write the table\n\nlater: rung three\n", "then write the table", "rung three"},
		"a numbered item": {
			"1. rung two: score the cells\n   then write the table\n2. rung three\n", "then write the table", "rung three"},
	} {
		t.Run(name, func(t *testing.T) {
			body := "## Done\nd\n## Now\nn\n## Next\n" + c.next + "\nPage: https://claude.ai/x\n"
			al := alertOfItem(t, store.IntentCheckpoint, body)
			text := words(al.Text)
			if al.Kind != worker.AlertCheckpoint || !strings.Contains(text, "rung two: score the cells") ||
				!strings.Contains(text, c.want) || strings.Contains(text, c.not) || strings.Contains(text, "Page:") {
				t.Fatalf("alert = %+v", al)
			}
			if al.Page != "https://claude.ai/x" {
				t.Fatalf("page = %q", al.Page)
			}
		})
	}
}

// Rejects a cut by bytes or at a fixed rune count (mid-word, the field's "Ship i"), a cut that
// ignores the last sentence end within the cap, a cut not marked with "…", a cut of a text that
// fits, and the old 200-rune cap.
func TestAlertTextIsCutAtASentenceOrWord(t *testing.T) {
	// covers: R-WRK-080, R-WRK-082
	first := strings.Repeat("Les données sont prêtes ", 6) + "maintenant. "        // 156 runes
	second := strings.Repeat("Faut-il publier la version ", 7) + "aujourd'hui? "   // 202 runes
	third := strings.Repeat("Le reste attend la décision finale ", 5) + "demain. " // 183 runes
	wordy := strings.Repeat("café crème ", 50)                                     // 550 runes, no sentence end
	fits := strings.Repeat("ünïcode ", 49) + "fin."                                // 396 runes
	t.Run("at the last sentence end within the cap", func(t *testing.T) {
		text := alertOfItem(t, store.IntentRequest, first+second+third).Text
		want := strings.TrimSpace(first + second)
		if !strings.HasSuffix(text, "…") || strings.TrimSpace(strings.TrimSuffix(text, "…")) != want {
			t.Fatalf("text = %q\nwant %q…", text, want)
		}
	})
	t.Run("at the last space without a sentence end", func(t *testing.T) {
		text := alertOfItem(t, store.IntentRequest, wordy).Text
		kept, cut := strings.CutSuffix(text, "…")
		kept = strings.TrimSpace(kept)
		n := utf8.RuneCountInString(kept)
		if !cut || n > alertCap || n < alertCap-12 || !strings.HasPrefix(wordy, kept) ||
			wordy[len(kept)] != ' ' {
			t.Fatalf("text of %d runes = %q", n, text)
		}
	})
	t.Run("a text that fits is whole", func(t *testing.T) {
		if text := alertOfItem(t, store.IntentRequest, fits).Text; text != fits {
			t.Fatalf("text = %q, want it whole", text)
		}
	})
	t.Run("exactly the cap is whole", func(t *testing.T) {
		exact := strings.Repeat("abcd ", 79) + "abcde" // 400 runes
		if text := alertOfItem(t, store.IntentRequest, exact).Text; text != exact {
			t.Fatalf("text = %q, want it whole", text)
		}
	})
	t.Run("a single word longer than the cap is cut at the cap", func(t *testing.T) {
		word := strings.Repeat("é", alertCap+1)
		if text := alertOfItem(t, store.IntentRequest, word).Text; text != strings.Repeat("é", alertCap)+"…" {
			t.Fatalf("text of %d runes = %q", utf8.RuneCountInString(text), text)
		}
	})
	t.Run("a sentence end in the first half loses to the last space", func(t *testing.T) {
		text := alertOfItem(t, store.IntentRequest, "Ok. "+wordy).Text
		kept, cut := strings.CutSuffix(text, "…")
		if n := utf8.RuneCountInString(kept); !cut || n < alertCap-12 || n > alertCap {
			t.Fatalf("text of %d runes = %q", n, text)
		}
	})
	t.Run("a long Ask is cut the same way", func(t *testing.T) {
		body := "## Done\nd\n## Now\nn\n## Next\nx\n## Ask\n" + first + "\n" + second + "\n" + third + "\n"
		al := alertOfItem(t, store.IntentCheckpoint, body)
		kept, cut := strings.CutSuffix(al.Text, "…")
		if !cut || words(kept) != words(first+second) {
			t.Fatalf("text = %q", al.Text)
		}
	})
}
