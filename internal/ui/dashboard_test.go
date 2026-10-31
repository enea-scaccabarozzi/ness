package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

// fakeLoad serves snapshots in order and records the all flag of each call.
type fakeLoad struct {
	snaps []ui.Snapshot
	errs  []error
	calls []bool
}

func (f *fakeLoad) load(all bool) (ui.Snapshot, error) {
	i := len(f.calls)
	f.calls = append(f.calls, all)
	if i < len(f.errs) && f.errs[i] != nil {
		return ui.Snapshot{}, f.errs[i]
	}
	return f.snaps[min(i, len(f.snaps)-1)], nil
}

func rows(lines ...string) []ui.Row {
	var out []ui.Row
	for _, l := range lines {
		out = append(out, ui.Row{Line: l, Detail: "detail of " + l})
	}
	return out
}

// run executes cmd and feeds its message back, as the program would.
func run(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	m, _ = m.Update(cmd())
	return m
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func view(m tea.Model) string { return ansi.Strip(m.View().Content) }

func TestDashboardSelectsAndClamps(t *testing.T) {
	// covers: R-UI-028
	f := &fakeLoad{snaps: []ui.Snapshot{{Header: "HEAD", Rows: rows("one", "two"), At: time.Unix(0, 0)}}}
	var m tea.Model = ui.NewDashboard(ui.Settings{Rich: true, Width: 80}.Theme(), 2*time.Second, f.load)
	m = run(t, m, m.Init())
	if v := view(m); !strings.Contains(v, "HEAD") || !strings.Contains(v, "› one") || !strings.Contains(v, "detail of one") {
		t.Fatalf("first view:\n%s", v)
	}
	for range 3 { // past the last row: stays on it
		m, _ = m.Update(key("down"))
	}
	if v := view(m); !strings.Contains(v, "› two") || !strings.Contains(v, "detail of two") {
		t.Fatalf("after down:\n%s", v)
	}
	for range 3 {
		m, _ = m.Update(key("up"))
	}
	if v := view(m); !strings.Contains(v, "› one") {
		t.Fatalf("after up:\n%s", v)
	}
	if !m.View().AltScreen {
		t.Fatal("the live view is not full-screen")
	}
}

func TestDashboardTogglesClosedAndRefreshes(t *testing.T) {
	// covers: R-UI-028
	f := &fakeLoad{snaps: []ui.Snapshot{{Rows: rows("open")}, {Rows: rows("open", "closed")}}}
	var m tea.Model = ui.NewDashboard(ui.Settings{Rich: true, Width: 80}.Theme(), time.Second, f.load)
	m = run(t, m, m.Init())
	var cmd tea.Cmd
	m, cmd = m.Update(key("a"))
	m = run(t, m, cmd)
	if len(f.calls) != 2 || f.calls[0] || !f.calls[1] {
		t.Fatalf("load calls = %v, want [false true]", f.calls)
	}
	if v := view(m); !strings.Contains(v, "closed") || !strings.Contains(v, "hide closed") {
		t.Fatalf("after a:\n%s", v)
	}
	m, cmd = m.Update(key("r"))
	_ = run(t, m, cmd)
	if len(f.calls) != 3 || !f.calls[2] {
		t.Fatalf("r did not reload with closed sessions: %v", f.calls)
	}
}

func TestDashboardKeepsTheLastViewOnAFailedLoad(t *testing.T) {
	// covers: R-UI-028
	f := &fakeLoad{snaps: []ui.Snapshot{{Rows: rows("kept")}}, errs: []error{nil, errors.New("store locked")}}
	var m tea.Model = ui.NewDashboard(ui.Settings{Rich: true, Width: 80}.Theme(), time.Second, f.load)
	m = run(t, m, m.Init())
	var cmd tea.Cmd
	m, cmd = m.Update(key("r"))
	m = run(t, m, cmd)
	v := view(m)
	if !strings.Contains(v, "kept") || !strings.Contains(v, "refresh failed: store locked") {
		t.Fatalf("after a failed load:\n%s", v)
	}
}

func TestDashboardQuits(t *testing.T) {
	// covers: R-UI-028
	m := ui.NewDashboard(ui.Settings{Rich: true}.Theme(), time.Second, (&fakeLoad{snaps: []ui.Snapshot{{}}}).load)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q does not quit")
	}
}

func TestDashboardKeepsOneRefreshLoop(t *testing.T) {
	// A refresh started by a key supersedes the running loop: the older load's result and its
	// tick are dropped, so presses never multiply the reloads.
	// covers: R-UI-028
	f := &fakeLoad{snaps: []ui.Snapshot{{Rows: rows("old")}, {Rows: rows("new")}}}
	var m tea.Model = ui.NewDashboard(ui.Settings{Rich: true, Width: 80}.Theme(), time.Second, f.load)
	first := m.Init()
	m, second := m.Update(key("r"))
	staleMsg, freshMsg := first(), second()
	m, cmd := m.Update(staleMsg)
	if cmd != nil || strings.Contains(view(m), "old") {
		t.Fatalf("a superseded load was shown or scheduled a refresh:\n%s", view(m))
	}
	m, tickCmd := m.Update(freshMsg)
	if tickCmd == nil || !strings.Contains(view(m), "new") {
		t.Fatalf("the latest load was not shown or scheduled nothing:\n%s", view(m))
	}
}
