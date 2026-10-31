package profiles_test

import (
	"strings"
	"testing"
)

// Rejects ness-entry text that leaves the lifecycle slots before the Full session heading (a quick
// root would read them), or the sizes slot outside Work sizes, or loses a size's heading.
func TestEntryPlacesCeremonyUnderItsSize(t *testing.T) {
	// covers: R-BLT-111
	md := entrySkill(t)
	at := func(s string) int {
		t.Helper()
		i := strings.Index(md, s)
		if i < 0 {
			t.Fatalf("ness-entry lacks %q", s)
		}
		return i
	}
	sizes, notIn, quick, full := at("\n## Work sizes\n"), at("\n## Not in a session yet\n"),
		at("\n## Quick root\n"), at("\n## Full session\n")
	child, compact := at("\n## Child\n"), at("\n## After compaction\n")
	if !(sizes < notIn && notIn < quick && quick < full && full < child && child < compact) {
		t.Fatalf("headings out of order: %d %d %d %d %d %d", sizes, notIn, quick, full, child, compact)
	}
	if i := at("<!-- ness:slot list sizes -->"); i < sizes || i > notIn {
		t.Errorf("the sizes slot at %d is outside Work sizes (%d..%d)", i, sizes, notIn)
	}
	for _, slot := range []string{"specify", "plan", "handoff"} {
		if i := at("<!-- ness:slot list " + slot + " -->"); i < full || i > child {
			t.Errorf("the %s slot at %d is outside Full session (%d..%d)", slot, i, full, child)
		}
	}
	for _, want := range []string{"ness size full", "ness open", "done", "one checkpoint"} {
		if !strings.Contains(md[quick:full], want) {
			t.Errorf("Quick root does not say %q", want)
		}
	}
}

// Rejects ness-entry text that hides from a quick root what every size owes: the report with its
// Lessons heading and the operator-worded root close sit in Closing, before the Quick root and
// Full session sections a reader skips.
func TestEntryClosingAppliesToEverySize(t *testing.T) {
	// covers: R-BLT-111
	md := entrySkill(t)
	closing := section(t, md, "Closing")
	for _, want := range []string{"## Lessons", "--authorized", "report.md"} {
		if !strings.Contains(closing, want) {
			t.Errorf("Closing lacks %q:\n%s", want, closing)
		}
	}
	if strings.Index(md, "\n## Closing\n") > strings.Index(md, "\n## Quick root\n") {
		t.Error("Closing comes after Quick root")
	}
	full, child := strings.Index(md, "\n## Full session\n"), strings.Index(md, "\n## Child\n")
	if strings.Contains(md[full:child], "--authorized") {
		t.Error("Full session still holds the root close rule")
	}
}

// Rejects the promote command that omits --size and an unbound note that does not name it.
func TestEntryPromotionNamesTheSize(t *testing.T) {
	// covers: R-BLT-111
	_, body, ok := strings.Cut(entrySkill(t), "\n## Not in a session yet\n")
	if !ok {
		t.Fatal("no Not in a session yet section")
	}
	body, _, _ = strings.Cut(body, "\n## ")
	if !strings.Contains(strings.Join(strings.Fields(body), " "), "--tier TIER --size SIZE --brief FILE") {
		t.Errorf("the promote command lacks --size:\n%s", body)
	}
}
