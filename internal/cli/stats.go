package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/ladder"
	"github.com/enea-scaccabarozzi/ness/internal/stats"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

var statsGroups = []string{"session", "tier", "role", "rung", "tool"}

type statsOpts struct {
	since, by string
	asJSON    bool
}

func newStatsCmd(d Deps) *cobra.Command {
	var o statsOpts
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Tokens, context, compactions, tool bytes and cost of this project's sessions, from their transcripts",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runStats(d, o) },
	}
	f := cmd.Flags()
	f.StringVar(&o.since, "since", "", "only entries at or after `WHEN`: a date, an RFC 3339 time or a duration (7d)")
	f.StringVar(&o.by, "by", "session", "group by session | tier | role | rung | tool")
	f.BoolVar(&o.asJSON, "json", false, "print one JSON object")
	return cmd
}

// statsRow is one line of stats; the JSON field schema is frozen by stats_json.txtar (R-CFG-037).
type statsRow struct {
	Key              string      `json:"key"`
	Sessions         int         `json:"sessions"`
	Turns            int         `json:"turns"`
	Compactions      int         `json:"compactions"`
	ContextTokens    int         `json:"context_tokens"`
	InputTokens      int         `json:"input_tokens"`
	OutputTokens     int         `json:"output_tokens"`
	CacheReadTokens  int         `json:"cache_read_tokens"`
	CacheWriteTokens int         `json:"cache_write_tokens"`
	CostCents        int         `json:"cost_cents"`
	Unpriced         bool        `json:"unpriced"`
	SkippedLines     int         `json:"skipped_lines"`
	ToolBytes        int         `json:"tool_bytes"`
	RouterReason     string      `json:"router_reason"`
	Spawns           spawnCounts `json:"spawns"`
}

// spawnCounts is the pre-tool hook's decisions on subagent calls, by verdict (R-STAT-020).
type spawnCounts struct {
	Allowed     int `json:"allowed"`
	Below       int `json:"below"`
	Escalated   int `json:"escalated"`
	Refused     int `json:"refused"`
	WouldRefuse int `json:"would_refuse"`
	Fork        int `json:"fork"`
}

func (c *spawnCounts) add(verdict string) {
	switch verdict {
	case "allowed":
		c.Allowed++
	case "below":
		c.Below++
	case "escalated":
		c.Escalated++
	case "refused":
		c.Refused++
	case "would-refuse":
		c.WouldRefuse++
	case "fork":
		c.Fork++
	}
}

func (c *spawnCounts) sum(o spawnCounts) {
	c.Allowed += o.Allowed
	c.Below += o.Below
	c.Escalated += o.Escalated
	c.Refused += o.Refused
	c.WouldRefuse += o.WouldRefuse
	c.Fork += o.Fork
}

type notCountedJSON struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type statsJSON struct {
	Schema     int              `json:"schema"`
	By         string           `json:"by"`
	Since      string           `json:"since"`
	PriceDate  string           `json:"price_date"`
	Rows       []statsRow       `json:"rows"`
	Total      statsRow         `json:"total"`
	NotCounted []notCountedJSON `json:"not_counted"`
}

// figure is one counted session: T is the session with its subagents, Own its own turns alone
// and Subs each subagent's, keyed by rung (R-STAT-020).
type figure struct {
	ID, Tier, Role, Reason string
	T                      stats.Transcript
	Cents                  int
	Unpriced               bool
	Own                    part
	Subs                   []part
	Spawns                 spawnCounts
}

// part is one transcript file of a session, priced.
type part struct {
	Rung     string
	T        stats.Transcript
	Cents    int
	Unpriced bool
}

func newPart(rung string, t stats.Transcript) part {
	c, u := t.Cost()
	return part{Rung: rung, T: t, Cents: c, Unpriced: u}
}

func runStats(d Deps, o statsOpts) error {
	if !slices.Contains(statsGroups, o.by) {
		return usage("--by %q: accepted values are %s", o.by, strings.Join(statsGroups, ", "))
	}
	var since time.Time
	if o.since != "" {
		t, err := stats.ParseSince(o.since, d.Now())
		if err != nil {
			return usage("%v", err)
		}
		since = t
	}
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	path, err := store.Locate(p.root)
	if err != nil {
		return refuse("%v", err)
	}
	primary := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	rows, spawns, err := statsSessions(path)
	if err != nil {
		return err
	}
	rung, err := subagentRung(d, p, o.by)
	if err != nil {
		return err
	}
	spawns = slices.DeleteFunc(spawns, func(sp store.Spawn) bool { return sp.At.Before(since) })
	figs, nc := collectStats(rows, primary, since, rung)
	out := groupStats(figs, nc, o.by, spawns)
	if o.since != "" {
		out.Since = since.UTC().Format(time.RFC3339)
	}
	if o.asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	printStats(d, out)
	return nil
}

// subagentRung names a subagent's rung from its meta for --by rung: its model/effort on the
// project's ladder; with neither, a named agent's rendered rung, else the session's tier
// (a fork); off-ladder otherwise. Other groupings need no rung and no config.
func subagentRung(d Deps, p *project, by string) (func(m stats.Meta, tier string) string, error) {
	if by != "rung" {
		return func(stats.Meta, string) string { return "" }, nil
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, err
	}
	l := ladder.New(cfg, "claude")
	return func(m stats.Meta, tier string) string {
		if m.Model == "" && m.Effort == "" {
			if i, ok := namedRung(m.AgentType, cfg, l); ok {
				return l.Rungs[i].Name
			}
			return tier
		}
		if i, ok := l.Find(m.Model, m.Effort); ok {
			return l.Rungs[i].Name
		}
		return "off-ladder"
	}, nil
}

// statsSessions reads every session, closed ones included (R-STAT-012), and every spawn
// decision, without writing (R-STAT-003): no store means none; a store of another schema
// version is refused.
func statsSessions(path string) ([]store.Session, []store.Spawn, error) {
	s, err := store.OpenReadOnly(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil, nil
	case err != nil: // older or newer schema, as doctor and status refuse them
		return nil, nil, refuse("%v", err)
	}
	defer func() { _ = s.Close() }() //nolint:errcheck // read-only store
	rows, err := s.Sessions(true)
	if err != nil {
		return nil, nil, err
	}
	spawns, err := s.Spawns("")
	return rows, spawns, err
}

// collectStats reads each bound Claude Code transcript; codex sessions (R-STAT-013) and
// sessions without a transcript are listed as not counted.
// rung names the rung of a subagent from its meta.
func collectStats(
	rows []store.Session, primary string, since time.Time, rung func(stats.Meta, string) string,
) ([]figure, []notCountedJSON) {
	var figs []figure
	var nc []notCountedJSON
	for _, sess := range rows {
		if sess.Client == "codex" {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "codex: not counted in v1"})
			continue
		}
		if sess.ClientSession == "" {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "no transcript"})
			continue
		}
		path := transcriptPath(primary, sess)
		tr, rerr := readTranscript(path, since)
		if errors.Is(rerr, fs.ErrNotExist) {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "no transcript"})
			continue
		}
		if rerr != nil {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "unreadable transcript: " + rerr.Error()})
			continue
		}
		tier := cmp.Or(sess.Tier, "-")
		own := tr
		own.Models, own.ToolBytes = maps.Clone(tr.Models), maps.Clone(tr.ToolBytes)
		subs := addSubagents(&tr, strings.TrimSuffix(path, ".jsonl"), since,
			func(m stats.Meta) string { return rung(m, tier) })
		fig := figure{
			ID: sess.ID, Tier: tier, Role: "child", Reason: sess.RouterReason,
			T: tr, Own: newPart(tier, own), Subs: subs,
		}
		if sess.Parent == store.Operator {
			fig.Role = "root"
		}
		fig.Cents, fig.Unpriced = tr.Cost()
		figs = append(figs, fig)
	}
	return figs, nc
}

func (r *statsRow) add(p part) {
	r.Turns += len(p.T.Turns)
	r.Compactions += p.T.Compactions
	r.ContextTokens = max(r.ContextTokens, p.T.Context())
	r.InputTokens += p.T.Usage.Input
	r.OutputTokens += p.T.Usage.Output
	r.CacheReadTokens += p.T.Usage.CacheRead
	r.CacheWriteTokens += p.T.Usage.CacheWrite
	r.CostCents += p.Cents
	r.Unpriced = r.Unpriced || p.Unpriced
	r.SkippedLines += p.T.Skipped
	for _, b := range p.T.ToolBytes {
		r.ToolBytes += b
	}
}

// groupStats builds the rows of one dimension and the total, which is the same for every
// dimension (R-STAT-005). Tool rows carry bytes per tool (R-STAT-011). Rung and role rows split
// a session's own turns from its subagents' (R-STAT-020). The total's spawns count every
// decision, sessions without a transcript included.
func groupStats(figs []figure, nc []notCountedJSON, by string, spawns []store.Spawn) statsJSON {
	out := statsJSON{Schema: 1, By: by, PriceDate: stats.PriceDate, Rows: []statsRow{}, Total: statsRow{Key: "total"}}
	if nc == nil {
		nc = []notCountedJSON{}
	}
	out.NotCounted = nc
	perSession := map[string]spawnCounts{}
	for _, sp := range spawns {
		out.Total.Spawns.add(sp.Verdict)
		c := perSession[sp.Session]
		c.add(sp.Verdict)
		perSession[sp.Session] = c
	}
	groups := map[string]*statsRow{}
	for _, f := range figs {
		out.Total.Sessions++
		out.Total.add(part{T: f.T, Cents: f.Cents, Unpriced: f.Unpriced})
		if by == "tool" {
			for name, b := range f.T.ToolBytes {
				row := group(groups, name)
				row.ToolBytes += b
			}
			continue
		}
		key := map[string]string{"session": f.ID, "tier": f.Tier, "role": f.Role, "rung": f.Tier}[by]
		row := group(groups, key)
		row.Sessions++
		row.Spawns.sum(perSession[f.ID])
		if by == "rung" || by == "role" {
			splitSubagents(groups, row, f, by)
			continue
		}
		row.add(part{T: f.T, Cents: f.Cents, Unpriced: f.Unpriced})
		if by == "session" {
			row.RouterReason = f.Reason
		}
	}
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		out.Rows = append(out.Rows, *groups[key])
	}
	return out
}

// splitSubagents adds f's own turns to row and each subagent's to its rung's row, or to the
// subagent row by role.
func splitSubagents(groups map[string]*statsRow, row *statsRow, f figure, by string) {
	row.add(f.Own)
	for _, s := range f.Subs {
		key := s.Rung
		if by == "role" {
			key = "subagent"
		}
		group(groups, key).add(s)
	}
}

func group(groups map[string]*statsRow, key string) *statsRow {
	if groups[key] == nil {
		groups[key] = &statsRow{Key: key}
	}
	return groups[key]
}

func printStats(d Deps, out statsJSON) {
	if d.rich() {
		printRichStats(d, out)
		return
	}
	if out.By == "tool" {
		say(d.Stdout, "tool\tbytes")
		for _, r := range out.Rows {
			say(d.Stdout, "%s\t%d", r.Key, r.ToolBytes)
		}
	} else {
		say(d.Stdout, "%s\tsessions\tturns\tcompactions\tcontext\tinput\toutput\tcache_read\tcache_write\tcost", out.By)
		for _, r := range out.Rows {
			say(d.Stdout, "%s", statsLine(r))
		}
	}
	say(d.Stdout, "%s", statsLine(out.Total))
	for _, n := range out.NotCounted {
		say(d.Stdout, "not counted\t%s\t%s", n.ID, n.Reason)
	}
	say(d.Stdout, "prices\t%s", stats.PriceDate)
}

// printRichStats is the same rows as a table, numbers shortened, the total last (R-UI-023).
func printRichStats(d Deps, out statsJSON) {
	t, w := d.theme(), d.out()
	tb := ui.Table{Total: true}
	if out.By == "tool" {
		tb.Headers, tb.Right = []string{"tool", "bytes"}, []int{1}
		for _, r := range append(out.Rows, out.Total) {
			tb.Rows = append(tb.Rows, []string{r.Key, short(r.ToolBytes)})
		}
	} else {
		tb.Headers = []string{out.By, "sess", "turns", "compact", "ctx", "in", "out", "cache rd", "cache wr", "cost"}
		tb.Right = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
		for _, r := range append(out.Rows, out.Total) {
			tb.Rows = append(tb.Rows, []string{
				r.Key, short(r.Sessions), short(r.Turns), short(r.Compactions),
				short(r.ContextTokens), short(r.InputTokens), short(r.OutputTokens), short(r.CacheReadTokens),
				short(r.CacheWriteTokens), t.Accent.Render(cost(r)),
			})
		}
	}
	say(w, "%s", t.Render(tb))
	for _, n := range out.NotCounted {
		say(w, "%s %s %s", t.Warn.Render(ui.GlyphWarn+" not counted"), n.ID, t.Dim.Render(n.Reason))
	}
	say(w, "%s", t.Dim.Render("prices as of "+stats.PriceDate))
}

// short is n with a k or M suffix past a thousand.
func short(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

func cost(r statsRow) string {
	c := fmt.Sprintf("$%d.%02d", r.CostCents/100, r.CostCents%100)
	if r.Unpriced {
		c += "+ (unpriced model: lower bound)"
	}
	return c
}

func statsLine(r statsRow) string {
	return fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s", r.Key, r.Sessions, r.Turns, r.Compactions,
		r.ContextTokens, r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens, cost(r))
}

func readTranscript(path string, since time.Time) (stats.Transcript, error) {
	f, err := os.Open(path) //nolint:gosec // the client's transcript of this session
	if err != nil {
		return stats.Transcript{}, err
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // read-only file
	return stats.Read(f, since)
}

// addSubagents adds the usage of <session>/subagents/*.jsonl, where Claude Code writes the
// calls of a session's subagents. Message ids are per file: nothing is deduplicated across
// files. Their turns and compactions are not the session's own. A file that cannot be read
// (a directory, a dangling link, one that vanished, an over-long line) is skipped and counted
// in the session's skipped figure; the session keeps its other figures.
// Each file read is returned as a part, its rung named from the meta beside it.
func addSubagents(tr *stats.Transcript, sessionDir string, since time.Time, rung func(stats.Meta) string) []part {
	entries, err := os.ReadDir(filepath.Join(sessionDir, "subagents"))
	if err != nil {
		return nil // no subagents directory, or an unreadable one: nothing to add
	}
	var parts []part
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if e.Type()&(fs.ModeNamedPipe|fs.ModeSocket|fs.ModeDevice) != 0 {
			tr.Skipped++ // opening a pipe would block
			continue
		}
		sub, rerr := readTranscript(filepath.Join(sessionDir, "subagents", e.Name()), since)
		if rerr != nil {
			tr.Skipped++
			continue
		}
		tr.Usage = tr.Usage.Add(sub.Usage)
		if tr.Models == nil {
			tr.Models = map[string]stats.Usage{}
		}
		for m, u := range sub.Models {
			tr.Models[m] = tr.Models[m].Add(u)
		}
		for name, b := range sub.ToolBytes {
			tr.ToolBytes[name] += b
		}
		tr.Skipped += sub.Skipped
		meta := readMeta(filepath.Join(sessionDir, "subagents", strings.TrimSuffix(e.Name(), ".jsonl")+".meta.json"))
		parts = append(parts, newPart(rung(meta), sub))
	}
	return parts
}

// readMeta is a subagent's meta; empty when it is missing or unreadable, which no rung matches.
func readMeta(path string) stats.Meta {
	f, err := os.Open(path) //nolint:gosec // the client's meta beside a transcript
	if err != nil {
		return stats.Meta{}
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // read-only file
	m, err := stats.ReadMeta(f)
	if err != nil {
		return stats.Meta{}
	}
	return m
}
