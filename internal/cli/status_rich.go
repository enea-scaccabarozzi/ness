package cli

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

// statusHeader is what rich status shows above the tree.
type statusHeader struct {
	project, base string
	probesOn      bool
}

// printRichStatus is status drawn: the tree, the probes and the worker (R-UI-027).
func printRichStatus(d Deps, out statusJSON, h statusHeader) {
	snap := richStatus(d, out, h)
	w := d.out()
	say(w, "%s\n", snap.Header)
	if len(snap.Rows) == 0 {
		say(w, "%s", d.theme().Dim.Render("no open sessions · ness open --root --scope NAME --tier TIER --brief FILE"))
	}
	for _, r := range snap.Rows {
		say(w, "%s", r.Line)
	}
	say(w, "\n%s", snap.Footer)
}

// richStatus builds the drawn status: one row per session with its detail, and the probes and
// worker as the footer. The live view (status --watch) shows the same snapshot.
func richStatus(d Deps, out statusJSON, h statusHeader) ui.Snapshot {
	t := d.theme()
	snap := ui.Snapshot{
		Header: t.Badge("ness", "accent") + " " + t.Bold.Render(h.project) + t.Dim.Render(" · base ") + h.base +
			"   " + workerPill(t, out.Worker),
	}
	scopeW, tierW, phaseW := 0, 0, 0
	for _, j := range out.Sessions {
		scopeW, tierW, phaseW = max(scopeW, len(j.Scope)), max(tierW, len(tierSize(j))), max(phaseW, len(j.Phase))
	}
	prefixes, treeW := treePrefixes(out.Sessions), 0
	for _, pr := range prefixes {
		treeW = max(treeW, utf8.RuneCountInString(pr))
	}
	for i, j := range out.Sessions {
		ctx, idle := t.Dim.Render("ctx -"), t.Dim.Render("idle -")
		if j.ContextTokens != nil {
			ctx = "ctx " + t.Glyph(fmt.Sprintf("%dk", *j.ContextTokens/1000), contextColor(*j.ContextTokens))
		}
		if j.IdleMinutes != nil {
			idle = fmt.Sprintf("idle %dm", *j.IdleMinutes)
		}
		waits := t.Dim.Render("waits 0")
		if n := len(j.Obligations); n > 0 {
			waits = t.Warn.Render(fmt.Sprintf("waits %d", n)) + t.Dim.Render(" ("+strings.Join(waitedOn(j), ", ")+")")
		}
		pad := strings.Repeat(" ", treeW-utf8.RuneCountInString(prefixes[i]))
		line := fmt.Sprintf("%s%s%s %s  %-*s  %-*s  %s  %s  %s  %s",
			t.Dim.Render(prefixes[i]), pad, phaseGlyph(t, j.Phase),
			t.Bold.Render(j.ID), scopeW, j.Scope, tierW, tierSize(j), phaseText(t, j.Phase, phaseW), ctx, idle, waits)
		if j.Closed {
			line = t.Dim.Render(prefixes[i] + "○ " + j.ID + "  " + j.Scope + "  closed")
		}
		snap.Rows = append(snap.Rows, ui.Row{Line: line, Detail: sessionDetail(t, j)})
	}
	snap.Footer = probeBlock(t, out.Probes, h.probesOn)
	return snap
}

// treePrefixes are the tree connectors of rows ordered depth-first (ordered).
func treePrefixes(rows []sessionJSON) []string {
	last := lastSiblings(rows)
	out := make([]string, len(rows))
	for i, r := range rows {
		if r.Depth == 0 {
			continue
		}
		var b strings.Builder
		for level := 1; level < r.Depth; level++ { // an ancestor with a later sibling draws a bar
			if k := ancestor(rows, i, level); k >= 0 && !last[k] {
				b.WriteString("│  ")
			} else {
				b.WriteString("   ")
			}
		}
		if last[i] {
			b.WriteString("└─ ")
		} else {
			b.WriteString("├─ ")
		}
		out[i] = b.String()
	}
	return out
}

// lastSiblings reports for each row that no later sibling follows it under the same parent.
func lastSiblings(rows []sessionJSON) []bool {
	last := make([]bool, len(rows))
	for i, r := range rows {
		last[i] = true
		for _, n := range rows[i+1:] {
			if n.Depth <= r.Depth {
				last[i] = n.Depth < r.Depth
				break
			}
		}
	}
	return last
}

// ancestor is the index of row i's ancestor at level, or -1.
func ancestor(rows []sessionJSON, i, level int) int {
	for k := i - 1; k >= 0; k-- {
		if rows[k].Depth == level {
			return k
		}
	}
	return -1
}

func waitedOn(j sessionJSON) []string {
	var from []string
	for _, o := range j.Obligations {
		if !slices.Contains(from, o.From) {
			from = append(from, o.From)
		}
	}
	return from
}

// phaseColor: ready and done are green, a stalled launch red, anything still starting amber.
func phaseColor(phase string) string {
	switch phase {
	case store.PhaseReady, "done":
		return "ok"
	case "stalled launch":
		return "fail"
	case "closed":
		return "dim"
	}
	return "warn"
}

func phaseGlyph(t ui.Theme, phase string) string { return t.Glyph(ui.GlyphDot, phaseColor(phase)) }

func phaseText(t ui.Theme, phase string, width int) string {
	return t.Glyph(fmt.Sprintf("%-*s", width, phase), phaseColor(phase))
}

// contextColor colors a context size: green under 100k tokens, amber under 160k, red above.
func contextColor(tokens int) string {
	switch {
	case tokens >= 160_000:
		return "fail"
	case tokens >= 100_000:
		return "warn"
	}
	return "ok"
}

func sessionDetail(t ui.Theme, j sessionJSON) string {
	field := func(k, v string) string {
		if v == "" {
			v = "-"
		}
		return t.Dim.Render(fmt.Sprintf("%-12s", k)) + v
	}
	lines := make([]string, 0, 7+len(j.Obligations))
	lines = append(lines,
		t.Bold.Render(j.ID)+"  "+j.Scope+title(t, j),
		field("parent", j.Parent), field("client", j.Client+" "+j.Effort), field("branch", j.Branch),
		field("worktree", j.Worktree), field("pane", j.Pane), field("integrated", j.Integrated))
	for _, o := range j.Obligations {
		lines = append(lines, field("waits on", o.From+" "+t.Dim.Render(o.ID)))
	}
	return strings.Join(lines, "\n")
}

// title is the session's tab title when it says more than the scope.
func title(t ui.Theme, j sessionJSON) string {
	if j.Title == "" || j.Title == j.Scope {
		return ""
	}
	return t.Dim.Render("  " + j.Title)
}

func workerPill(t ui.Theme, w workerJSON) string {
	switch {
	case w.HeartbeatAgeSeconds == nil:
		return t.Pill("worker not running", "fail")
	case !w.Running:
		return t.Pill(fmt.Sprintf("worker not running (last beat %ds ago)", *w.HeartbeatAgeSeconds), "fail")
	}
	beat := t.Dim.Render(fmt.Sprintf(" · beat %ds", *w.HeartbeatAgeSeconds))
	return t.Pill("worker "+w.Version, "ok") + beat
}

// probeBlock is the probes as a table; a paused probe is red (R-SESS-083).
func probeBlock(t ui.Theme, probes []probeJSON, on bool) string {
	switch {
	case !on:
		return t.Dim.Render("probes: feature off")
	case len(probes) == 0:
		return t.Dim.Render("probes: none")
	}
	tb := ui.Table{Headers: []string{"probe", "owner", "state", "last", "evidence"}}
	for _, p := range probes {
		state := p.State
		if p.NeedsEdit {
			state += " needs-edit"
		}
		if p.FileError != "" {
			state += " file error"
		}
		if p.Verdict != "" {
			state += " verdict " + p.Verdict
		}
		color := "ok"
		if p.State == store.ProbePaused {
			color = "fail"
		}
		last := p.LastState
		if last == "" {
			last = "-"
		}
		name := probe.Name(p.Path, p.Label)
		tb.Rows = append(tb.Rows, []string{name, p.Owner, t.Pill(state, color), last, excerpt(p.Evidence)})
	}
	return t.Render(tb)
}
