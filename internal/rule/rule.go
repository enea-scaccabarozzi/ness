// Package rule is the registry of rule pointers: the skill and section each ness refusal
// enforces, in the single format "(skill NAME, section HEADING)" (R-RULE-001).
package rule

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Pointer names the skill and the heading of that skill that state a rule.
type Pointer struct {
	// Skill is the skill name, such as ness-entry.
	Skill string
	// Section is the exact text of a heading in the skill, at any level.
	Section string
}

var (
	fenceRE   = regexp.MustCompile("^ {0,3}(```+|~~~+)")
	headingRE = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.+?)\s*$`)
)

// String renders the pointer as "(skill NAME, section HEADING)".
func (p Pointer) String() string { return "(skill " + p.Skill + ", section " + p.Section + ")" }

// ID names one built-in refusal in the registry.
type ID string

// The registered refusals (R-RULE-002).
const (
	HookPolling    ID = "hook-polling"
	HookBackground ID = "hook-background"
	HookOutput     ID = "hook-output"
	HookSleepLoop  ID = "hook-sleep-loop"
	Explore        ID = "explore"
	Lessons        ID = "lessons"
	ChildCap       ID = "child-cap"
	Checkpoint     ID = "checkpoint"
	Handoff        ID = "handoff"
	Subject        ID = "subject"
	DirtyWorktree  ID = "dirty-worktree"
	RootClose      ID = "root-close"
	RootOpen       ID = "root-open"
	ReportMissing  ID = "report-missing"
	ContractGuard  ID = "contract-guard"
	GateGuard      ID = "gate-guard"
	LimitsGuard    ID = "limits-guard"
	Size           ID = "size"
	QuickRoot      ID = "quick-root"
	ModelEffort    ID = "model-effort"
)

// Entry maps one refusal to its pointer.
type Entry struct {
	// ID is the refusal.
	ID ID
	// Pointer is the rule it enforces.
	Pointer Pointer
}

// Table returns the registry as a fresh slice.
func Table() []Entry {
	p := func(skill, section string) Pointer { return Pointer{Skill: skill, Section: section} }
	return []Entry{
		{HookPolling, p("ness-entry", "Communicate")},
		{HookBackground, p("ness-entry", "Communicate")},
		{HookOutput, p("ness-entry", "Communicate")},
		{HookSleepLoop, p("ness-entry", "Communicate")},
		{Explore, p("ness-entry", "Communicate")},
		{Lessons, p("ness-entry", "Lessons")},
		{ChildCap, p("ness-launch", "Child or subagent")},
		{Checkpoint, p("ness-entry", "Communicate")},
		{Handoff, p("ness-entry", "Communicate")},
		{Subject, p("ness-launch", "Wait, then fold")},
		{DirtyWorktree, p("ness-launch", "Wait, then fold")},
		{RootClose, p("ness-entry", "Closing")},
		{RootOpen, p("ness-launch", "Open a child")},
		{ReportMissing, p("ness-entry", "Closing")},
		{ContractGuard, p("ness-contracts", "Frozen files")},
		{GateGuard, p("ness-entry", "Tests")},
		{LimitsGuard, p("ness-launch", "Child or subagent")},
		{Size, p("ness-entry", "Work sizes")},
		{QuickRoot, p("ness-entry", "Quick root")},
		{ModelEffort, p("ness-entry", "Model and effort")},
	}
}

// Of returns the pointer of id. It panics for an id that is not registered.
func Of(id ID) Pointer {
	for _, e := range Table() {
		if e.ID == id {
			return e.Pointer
		}
	}
	panic(fmt.Sprintf("rule: no pointer registered for %q", id))
}

// Ref returns " " followed by the pointer of id, for appending to a message. It panics for an
// id that is not registered.
func Ref(id ID) string { return " " + Of(id).String() }

// Heading is one heading line of a markdown text.
type Heading struct {
	// Line is the zero-based line of the heading.
	Line int
	// Level is the number of leading #.
	Level int
	// Text is the heading text.
	Text string
}

// Headings returns the text of every heading in markdown, at any level, skipping fenced code.
func Headings(markdown []byte) []string {
	hs := HeadingLines(markdown)
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.Text)
	}
	return out
}

// HeadingLines returns every heading in markdown, at any level, skipping fenced code.
func HeadingLines(markdown []byte) []Heading {
	var out []Heading
	fence := ""
	for i, line := range strings.Split(string(markdown), "\n") {
		if m := fenceRE.FindString(line); m != "" {
			switch {
			case fence == "":
				fence = m[:3]
			case strings.HasPrefix(m, fence):
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if m := headingRE.FindStringSubmatch(line); m != nil {
			out = append(out, Heading{i, len(m[1]), strings.TrimSpace(m[2])})
		}
	}
	return out
}

// Resolve reports whether p lands on a heading: skills maps a skill name to its headings. The
// error names the missing skill or section and lists the headings that exist.
func Resolve(skills map[string][]string, p Pointer) error {
	headings, ok := skills[p.Skill]
	if !ok {
		return fmt.Errorf("skill %q does not ship", p.Skill)
	}
	if slices.Contains(headings, p.Section) {
		return nil
	}
	return fmt.Errorf("skill %q has no section %q; its sections: %s", p.Skill, p.Section, strings.Join(headings, ", "))
}
