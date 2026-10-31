package sync_test

import (
	"slices"
	"testing"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

func TestChangedSkills(t *testing.T) {
	// covers: R-UPD-070
	a := &nsync.Lock{Entries: []nsync.LockEntry{
		{Path: ".agents/skills/ness-entry/SKILL.md", Hash: "1"}, {Path: ".agents/skills/ness-launch/SKILL.md", Hash: "1"},
		{Path: ".agents/skills/ness-gone/SKILL.md", Hash: "1"}, {Path: "AGENTS.md", Hash: "1"},
	}}
	b := &nsync.Lock{Entries: []nsync.LockEntry{
		{Path: ".agents/skills/ness-entry/SKILL.md", Hash: "2"}, {Path: ".agents/skills/ness-launch/SKILL.md", Hash: "1"},
		{Path: ".agents/skills/ness-new/SKILL.md", Hash: "1"}, {Path: "AGENTS.md", Hash: "2"},
	}}
	if got := nsync.ChangedSkills(a, b); !slices.Equal(got, []string{"ness-entry", "ness-gone", "ness-new"}) {
		t.Fatal(got)
	}
}
