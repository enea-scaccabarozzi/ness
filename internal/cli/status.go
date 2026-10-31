package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

func newStatusCmd(d Deps) *cobra.Command {
	var all, asJSON, watch bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "The session tree: tier, phase, context, idle time, obligations; probes; the worker",
		Example: "  # the open sessions\n  ness status\n" +
			"  # a live view: select a session for its branch, worktree and waits\n  ness status --watch\n" +
			"  # one JSON object, for scripts\n  ness status --json",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if watch && (!d.rich() || !d.ui.Animate) {
				return refuse("--watch needs the rich renderer on a terminal (not NESS_UI=plain, a pipe or ACCESSIBLE)")
			}
			return withStore(d, func(s *store.Store, _ caller) error {
				if watch {
					return watchStatus(d, s)
				}
				return runStatus(d, s, all, asJSON)
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "also list closed sessions")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print one JSON object")
	cmd.Flags().BoolVar(&watch, "watch", false, "a live view, refreshed every 2s (rich renderer only)")
	cmd.MarkFlagsMutuallyExclusive("watch", "json")
	return cmd
}

type sessionJSON struct {
	ID            string           `json:"id"`
	Parent        string           `json:"parent"`
	Scope         string           `json:"scope"`
	Title         string           `json:"title"`
	Tier          string           `json:"tier"`
	Size          string           `json:"size"`
	Effort        string           `json:"effort"`
	Client        string           `json:"client"`
	Phase         string           `json:"phase"`
	Closed        bool             `json:"closed"`
	ContextTokens *int             `json:"context_tokens"`
	IdleMinutes   *int             `json:"idle_minutes"`
	Obligations   []obligationJSON `json:"obligations"`
	Waits         []waitJSON       `json:"waits"`
	Integrated    string           `json:"integrated"`
	Worktree      string           `json:"worktree"`
	Branch        string           `json:"branch"`
	Pane          string           `json:"pane"`
	ClientSession string           `json:"client_session"`
	Depth         int              `json:"depth"`
}

type obligationJSON struct {
	ID   string `json:"id"`
	From string `json:"from"`
}

// waitJSON is one thing a session waits on (R-WRK-099); Until is RFC 3339, "" when open-ended.
type waitJSON struct {
	Kind  string `json:"kind"`
	Ref   string `json:"ref"`
	Note  string `json:"note"`
	Until string `json:"until"`
}

type probeJSON struct {
	Path      string `json:"path"`
	Label     string `json:"label"`
	Owner     string `json:"owner"`
	State     string `json:"state"`
	LastState string `json:"last_state"`
	Evidence  string `json:"evidence"`
	NextRun   string `json:"next_run"`
	Verdict   string `json:"verdict"`
	NeedsEdit bool   `json:"needs_edit"`
	FileError string `json:"file_error"`
}

type workerJSON struct {
	Running             bool   `json:"running"`
	Version             string `json:"version"`
	HeartbeatAgeSeconds *int   `json:"heartbeat_age_seconds"`
}

type statusJSON struct {
	Schema   int           `json:"schema"`
	Sessions []sessionJSON `json:"sessions"`
	Probes   []probeJSON   `json:"probes"`
	Worker   workerJSON    `json:"worker"`
}

// workerRunningWithin is how fresh a heartbeat must be for the worker to count as running.
const workerRunningWithin = 60 * time.Second

func runStatus(d Deps, s *store.Store, all, asJSON bool) error {
	out, h, err := collectStatus(d, s, all, d.Now())
	if err != nil {
		return err
	}
	switch {
	case asJSON:
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	case d.rich():
		printRichStatus(d, out, h)
	default:
		printStatus(d, out, h.probesOn)
	}
	return nil
}

// watchStatus runs the live view until the operator quits (R-UI-028).
func watchStatus(d Deps, s *store.Store) error {
	load := func(all bool) (ui.Snapshot, error) {
		now := d.Now()
		out, h, err := collectStatus(d, s, all, now)
		if err != nil {
			return ui.Snapshot{}, err
		}
		snap := richStatus(d, out, h)
		snap.At = now
		return snap, nil
	}
	m := ui.NewDashboard(d.theme(), 2*time.Second, load)
	_, err := tea.NewProgram(m, tea.WithInput(d.Stdin), tea.WithOutput(d.Stdout)).Run()
	return err
}

// collectStatus reads everything status shows.
func collectStatus(d Deps, s *store.Store, all bool, now time.Time) (statusJSON, statusHeader, error) {
	out := statusJSON{Schema: 1, Sessions: []sessionJSON{}, Probes: []probeJSON{}}
	p, err := loadProject(d)
	if err != nil {
		return out, statusHeader{}, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return out, statusHeader{}, err
	}
	h := statusHeader{project: cfg.Project, base: cfg.Base, probesOn: cfg.On("probes")}
	primary, err := primaryRoot(p)
	if err != nil {
		return out, h, err
	}
	rows, err := s.Sessions(all)
	if err != nil {
		return out, h, err
	}
	for _, sess := range ordered(rows) {
		j := sessionView(d, sess, depth(rows, sess), primary, now, cfg.Launch.ReadyTimeout.Duration)
		if oerr := addObligations(s, &j); oerr != nil {
			return out, h, oerr
		}
		if werr := addWaits(s, &j, now); werr != nil {
			return out, h, werr
		}
		out.Sessions = append(out.Sessions, j)
	}
	if out.Probes, err = probeViews(s, primary, h.probesOn); err != nil {
		return out, h, err
	}
	out.Worker, err = workerView(s, now)
	return out, h, err
}

func addObligations(s *store.Store, j *sessionJSON) error {
	obl, err := s.Obligations(j.ID)
	for _, it := range obl {
		j.Obligations = append(j.Obligations, obligationJSON{ID: it.ID, From: displayName(it.Sender)})
	}
	return err
}

func addWaits(s *store.Store, j *sessionJSON, now time.Time) error {
	ws, err := s.Waits(j.ID, now)
	for _, w := range ws {
		until := ""
		if !w.Until.IsZero() {
			until = w.Until.UTC().Format(time.RFC3339)
		}
		j.Waits = append(j.Waits, waitJSON{Kind: w.Kind, Ref: w.Ref, Note: w.Note, Until: until})
	}
	return err
}

// waitsText is the `on ...` end of a status row: each wait's kind and ref, a park's until-time in
// UTC (R-WRK-099).
func waitsText(ws []waitJSON) string {
	parts := make([]string, 0, len(ws))
	for _, w := range ws {
		switch {
		case w.Kind != "parked":
			parts = append(parts, w.Kind+" "+w.Ref)
		case w.Until == "":
			parts = append(parts, "parked")
		default:
			until := w.Until
			if t, err := time.Parse(time.RFC3339, w.Until); err == nil {
				until = t.Format("01-02 15:04")
			}
			parts = append(parts, "parked until "+until)
		}
	}
	return "on " + strings.Join(parts, ", ")
}

// workerView reads the heartbeat; no heartbeat is a worker that never ran, any other error is real.
// Unpinned: no test can make the heartbeat read fail short of a corrupt store.
func workerView(s *store.Store, now time.Time) (workerJSON, error) {
	hb, err := s.Heartbeat()
	if errors.Is(err, store.ErrNoHeartbeat) {
		return workerJSON{}, nil
	}
	if err != nil {
		return workerJSON{}, err
	}
	age := int(now.Sub(hb.At).Seconds())
	running := now.Sub(hb.At) <= workerRunningWithin
	return workerJSON{Running: running, Version: hb.Version, HeartbeatAgeSeconds: &age}, nil
}

// sessionView is one session's row; context and idle time come from the client's transcript and
// stay nil when there is none (R-SESS-080).
func sessionView(d Deps, sess store.Session, depth int, primary string, now time.Time,
	timeout time.Duration,
) sessionJSON {
	j := sessionJSON{
		ID: sess.ID, Parent: displayName(sess.Parent), Scope: sess.Scope, Title: sess.Title,
		Tier: sess.Tier, Size: sess.Size, Effort: sess.Effort, Client: sess.Client,
		Phase:  session.Shown(sess, now, timeout),
		Closed: sess.Closed, Obligations: []obligationJSON{}, Waits: []waitJSON{}, Integrated: sess.Integrated,
		Worktree: sess.Worktree, Branch: sess.Branch, Pane: sess.Pane, ClientSession: sess.ClientSession,
		Depth: depth,
	}
	if sess.Closed {
		j.Phase = "closed"
		return j
	}
	if sess.ClientSession == "" {
		return j
	}
	if tr, ok := transcriptOf(d, sess, primary); ok {
		ctx, idle := tr.Context, int(now.Sub(tr.LastActive).Minutes())
		j.ContextTokens, j.IdleMinutes = &ctx, &idle
	}
	return j
}

// transcriptOf reads the session's transcript through its client, false when there is none.
// ponytail: reads each open session's whole transcript on every status; fine for a v1 tree,
// cache by size and mtime if status gets slow on long sessions.
func transcriptOf(d Deps, sess store.Session, primary string) (session.Transcript, bool) {
	if sess.ClientSession == "" {
		return session.Transcript{}, false
	}
	tr, err := buildClient(d, sess.Client).Transcript(userHome(), transcriptDir(primary, sess), sess.ClientSession)
	return tr, err == nil
}

// tierSize is the tier column of a row: a root adds its work size, a child nothing.
func tierSize(j sessionJSON) string {
	if j.Size == "" {
		return j.Tier
	}
	return j.Tier + " " + j.Size
}

// ordered returns the rows depth-first: each session followed by its children, in creation order.
func ordered(rows []store.Session) []store.Session {
	var out []store.Session
	var walk func(parent string)
	walk = func(parent string) {
		for _, r := range rows {
			if r.Parent == parent {
				out = append(out, r)
				walk(r.ID)
			}
		}
	}
	walk(store.Operator)
	return out
}

func depth(rows []store.Session, s store.Session) int {
	n := 0
	for s.Parent != store.Operator {
		i := slices.IndexFunc(rows, func(r store.Session) bool { return r.ID == s.Parent })
		if i < 0 {
			break
		}
		s, n = rows[i], n+1
	}
	return n
}

func printStatus(d Deps, out statusJSON, probesOn bool) {
	for _, j := range out.Sessions {
		ctx, idle := "-", "-"
		if j.ContextTokens != nil {
			ctx = fmt.Sprintf("%dk", *j.ContextTokens/1000)
		}
		if j.IdleMinutes != nil {
			idle = fmt.Sprintf("%dm", *j.IdleMinutes)
		}
		waits := fmt.Sprintf("waits %d", len(j.Obligations))
		if len(j.Obligations) > 0 {
			var from []string
			for _, o := range j.Obligations {
				if !slices.Contains(from, o.From) {
					from = append(from, o.From)
				}
			}
			waits += " (" + strings.Join(from, ", ") + ")"
		}
		if len(j.Waits) > 0 {
			waits += "\t" + waitsText(j.Waits)
		}
		say(d.Stdout, "%s%s\t%s\t%s\t%s\tctx %s\tidle %s\t%s",
			strings.Repeat("  ", j.Depth), j.ID, j.Scope, tierSize(j), j.Phase, ctx, idle, waits)
	}
	printProbes(d, out.Probes, probesOn)
	if out.Worker.HeartbeatAgeSeconds == nil {
		say(d.Stdout, "worker: not running")
		return
	}
	if !out.Worker.Running {
		say(d.Stdout, "worker: not running (last beat %ds ago)", *out.Worker.HeartbeatAgeSeconds)
		return
	}
	say(d.Stdout, "worker: %s heartbeat %ds", out.Worker.Version, *out.Worker.HeartbeatAgeSeconds)
}

// probeViews is every live probe as status shows it. A probe's file is read from its owner's
// checkout (P-19).
func probeViews(s *store.Store, primary string, on bool) ([]probeJSON, error) {
	if !on {
		return []probeJSON{}, nil
	}
	rows, err := s.Probes(false)
	if err != nil {
		return nil, err
	}
	sessions, err := s.Sessions(true)
	if err != nil {
		return nil, err
	}
	out := []probeJSON{}
	for _, row := range rows {
		var owner store.Session // zero for an unknown owner: the primary checkout
		if i := slices.IndexFunc(sessions, func(x store.Session) bool { return x.ID == row.Owner }); i >= 0 {
			owner = sessions[i]
		}
		j := probeJSON{
			Path: row.Path, Label: row.Label, Owner: row.Owner, State: row.State, LastState: row.LastState,
			Evidence: row.Evidence,
			Verdict:  row.Verdict, FileError: row.FileError,
		}
		if !row.NextRunAt.IsZero() {
			j.NextRun = row.NextRunAt.UTC().Format(time.RFC3339)
		}
		if row.NeedsEdit != "" {
			file := filepath.Join(ownerRoot(primary, owner), filepath.FromSlash(row.Path))
			data, rerr := os.ReadFile(file) //nolint:gosec // the owner's own probe file
			j.NeedsEdit = rerr == nil && row.NeedsEdit == probe.Hash(data)
		}
		out = append(out, j)
	}
	return out, nil
}

// printProbes is the probe lines of status: one per probe, a paused one red on a terminal
// (R-SESS-083).
func printProbes(d Deps, probes []probeJSON, on bool) {
	switch {
	case !on:
		say(d.Stdout, "probes: none (feature off)")
	case len(probes) == 0:
		say(d.Stdout, "probes: none")
	}
	for _, pj := range probes {
		state := pj.State
		if pj.NeedsEdit {
			state += " needs-edit"
		}
		if pj.FileError != "" {
			state += " file error"
		}
		if pj.Verdict != "" {
			state += " verdict " + pj.Verdict
		}
		if pj.State == store.ProbePaused {
			state = paint(d, state)
		}
		last := pj.LastState
		if last == "" {
			last = "-"
		}
		say(d.Stdout, "probe\t%s\t%s\t%s\t%s\t%s",
			probe.Name(pj.Path, pj.Label), pj.Owner, state, last, excerpt(pj.Evidence))
	}
}

// excerpt is the first 80 characters of the evidence on one line.
func excerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 80 {
		return string(r[:80])
	}
	return s
}

// paint wraps s in red when plain stdout is a terminal and NO_COLOR is unset (R-SESS-083).
func paint(d Deps, s string) string {
	if !d.Term.StdoutTTY || (d.Term.Getenv != nil && d.Term.Getenv("NO_COLOR") != "") {
		return s
	}
	return "\x1b[31m" + s + "\x1b[0m"
}
