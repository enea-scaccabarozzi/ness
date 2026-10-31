package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Row is one selectable line of the live view and the detail shown when it is selected.
type Row struct {
	Line, Detail string
}

// Snapshot is one load of the live view.
type Snapshot struct {
	Header string
	Rows   []Row
	Footer string
	// At is when the loader read it, shown in the key line.
	At time.Time
}

// Dashboard is the live status view (R-UI-028): it reloads every Every, keeps the last good
// snapshot when a load fails, and moves a selection over the rows.
type Dashboard struct {
	theme Theme
	load  func(all bool) (Snapshot, error)
	every time.Duration
	snap  Snapshot
	err   error
	sel   int
	all   bool
	gen   int // the latest load; an older load's result is dropped and schedules nothing
}

// NewDashboard returns the view; load is called with whether closed sessions are wanted.
func NewDashboard(t Theme, every time.Duration, load func(all bool) (Snapshot, error)) *Dashboard {
	return &Dashboard{theme: t, load: load, every: every}
}

type loaded struct {
	snap Snapshot
	err  error
	gen  int
}

type tick struct{ gen int }

// reload starts a new load; it supersedes any load still running and its refresh loop.
func (m *Dashboard) reload() tea.Cmd {
	m.gen++
	all, gen := m.all, m.gen
	return func() tea.Msg {
		s, err := m.load(all)
		return loaded{snap: s, err: err, gen: gen}
	}
}

// Init starts the first load.
func (m *Dashboard) Init() tea.Cmd { return m.reload() }

// Update handles loads, ticks and keys.
func (m *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loaded:
		if msg.gen != m.gen {
			return m, nil
		}
		m.err = msg.err
		if msg.err == nil {
			m.snap = msg.snap
			m.sel = min(m.sel, max(len(m.snap.Rows)-1, 0))
		}
		gen := m.gen
		return m, tea.Tick(m.every, func(time.Time) tea.Msg { return tick{gen: gen} })
	case tick:
		if msg.gen != m.gen {
			return m, nil
		}
		cmd := m.reload()
		return m, cmd
	case tea.KeyPressMsg:
		return m.key(msg.String())
	}
	return m, nil
}

// key handles one key press.
func (m *Dashboard) key(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		m.sel = max(m.sel-1, 0)
	case "down", "j":
		m.sel = min(m.sel+1, max(len(m.snap.Rows)-1, 0))
	case "a":
		m.all = !m.all
		cmd := m.reload()
		return m, cmd
	case "r":
		cmd := m.reload()
		return m, cmd
	}
	return m, nil
}

// View draws the header, the rows with the selection, its detail and the key help.
func (m *Dashboard) View() tea.View {
	t := m.theme
	out := []string{m.snap.Header, ""}
	if len(m.snap.Rows) == 0 {
		out = append(out, t.Dim.Render("  no open sessions"))
	}
	sel := lipgloss.NewStyle().Bold(true).Foreground(t.Color("accent"))
	for i, r := range m.snap.Rows {
		mark := "  "
		if i == m.sel {
			mark = sel.Render("› ")
		}
		out = append(out, mark+r.Line)
	}
	if m.sel < len(m.snap.Rows) && m.snap.Rows[m.sel].Detail != "" {
		box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Color("accent")).
			Padding(0, 1).MarginTop(1)
		out = append(out, box.Render(m.snap.Rows[m.sel].Detail))
	}
	if m.snap.Footer != "" {
		out = append(out, "", m.snap.Footer)
	}
	if m.err != nil {
		out = append(out, "", t.Fail.Render(GlyphFail+" refresh failed: "+m.err.Error()+"; showing the last view"))
	}
	closed := "show closed"
	if m.all {
		closed = "hide closed"
	}
	help := "↑/↓ select · a " + closed + " · r refresh · q quit"
	if !m.snap.At.IsZero() {
		help += " · updated " + m.snap.At.Format("15:04:05")
	}
	out = append(out, "", t.Dim.Render(help))
	v := tea.NewView(strings.Join(out, "\n"))
	v.AltScreen = true
	v.WindowTitle = "ness status"
	return v
}
