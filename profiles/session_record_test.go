package profiles_test

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/profiles"
)

// entrySkill returns the core ness-entry skill text.
func entrySkill(t *testing.T) string {
	t.Helper()
	b, err := fs.ReadFile(profiles.Core(), "skills/ness-entry/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// section returns the body of the "## name" or "### name" section, up to the next heading.
func section(t *testing.T, md, name string) string {
	t.Helper()
	_, rest, ok := strings.Cut(md, "\n## "+name+"\n")
	if !ok {
		_, rest, ok = strings.Cut(md, "\n### "+name+"\n")
	}
	if !ok {
		t.Fatalf("no ## %s section", name)
	}
	body, _, _ := strings.Cut(rest, "\n#")
	return body
}

var (
	rootIDs = regexp.MustCompile(`decision ids? (are|belongs? to|come from) the root`)
	plain   = regexp.MustCompile(`decisions\.md[^.]*plain dated notes[^.]*without ids`)
)

// Rejects Session record prose that lets a child mint decision ids or cite its notes upward
// (FF-D6), and one that leaves the Lessons heading close demands unexplained.
func TestSessionRecordSaysWhoOwnsDecisionIDs(t *testing.T) {
	// covers: R-BLT-109
	rec := strings.Join(strings.Fields(strings.ToLower(section(t, entrySkill(t), "Session record"))), " ")
	if !rootIDs.MatchString(rec) {
		t.Errorf("Session record does not say decision ids are the root's:\n%s", rec)
	}
	if !plain.MatchString(rec) {
		t.Errorf("Session record does not say a child's decisions.md is plain dated notes without ids:\n%s", rec)
	}
	if !strings.Contains(rec, "never cited upward") {
		t.Errorf("Session record does not say a child's notes are never cited upward:\n%s", rec)
	}
	closing := strings.ToLower(section(t, entrySkill(t), "Closing"))
	if !strings.Contains(closing, "## lessons") {
		t.Errorf("Closing does not name the ## Lessons heading of report.md:\n%s", closing)
	}
}

// Rejects a close refusal whose pointer cannot resolve: the skill needs a Lessons heading.
func TestEntrySkillHasALessonsHeading(t *testing.T) {
	// covers: R-BLT-109, R-SESS-153
	if !regexp.MustCompile(`(?m)^#{1,6} Lessons\s*$`).MatchString(entrySkill(t)) {
		t.Fatal("ness-entry has no Lessons heading for (skill ness-entry, section Lessons)")
	}
}
