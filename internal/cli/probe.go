package cli

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newProbeCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Start, run, answer or stop probes: scheduled questions about the running world",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var keep, asJSON bool
	var note, label string
	start := &cobra.Command{
		Use:   "start PATH...",
		Short: "Register probe files or a directory of them; you own them",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !store.ValidLabel(label) {
				return usage("--label %q: a label matches [A-Za-z0-9][A-Za-z0-9._-]{0,31}", label)
			}
			return runProbeStart(d, args, label, keep)
		},
	}
	start.Flags().BoolVar(&keep, "keep-window", false, "on a restart, keep the original window start")
	start.Flags().StringVar(&label, "label", "",
		"name this start (a release, a sha) so the same file can run once per label; the floor sees NESS_PROBE_LABEL")
	run := &cobra.Command{
		Use:   "run PATH",
		Short: "Run the floor now and print the state and evidence; record nothing",
		Args:  cobra.ExactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return runProbeRun(d, args[0], asJSON) },
	}
	run.Flags().BoolVar(&asJSON, "json", false, "print one JSON object")
	stop := &cobra.Command{
		Use:   "stop PATH",
		Short: "Stop your probe at PATH (--label X when it runs under several labels) and record why",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if note == "" {
				return usage("--note is required: why the probe stops")
			}
			return runProbeStop(d, args[0], label, note)
		},
	}
	stop.Flags().StringVar(&note, "note", "", "why the probe stops (recorded)")
	stop.Flags().StringVar(&label, "label", "", "the label the probe was started with; needed when several are live")
	cmd.AddCommand(start, run, stop, newVerdictCmd(d))
	return cmd
}

type verdictOpts struct {
	state, note string
	bug         bool
}

func newVerdictCmd(d Deps) *cobra.Command {
	var o verdictOpts
	cmd := &cobra.Command{
		Use:   "verdict ITEM",
		Short: "Answer a probe wake with green or red; posts it when the probe is judged",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if o.state != "green" && o.state != "red" {
				return usage("--state %q: accepted values are green, red", o.state)
			}
			if o.note == "" {
				return usage("--note is required: what you found")
			}
			return runVerdict(d, args[0], o)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.state, "state", "", "green | red")
	f.StringVar(&o.note, "note", "", "your finding, posted when the probe is judged")
	f.BoolVar(&o.bug, "probe-bug", false, "the probe itself is wrong: record a defect, post nothing, flag needs-edit")
	return cmd
}

// runVerdict answers the wake (R-PRB-064): the item is disposed without a reply, the verdict
// recorded, and posted when the file says judged unless it is a probe bug (R-PRB-065).
func runVerdict(d Deps, item string, o verdictOpts) (err error) {
	p, cfg, err := requireFeature(d, "probes")
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	return answerVerdict(d, cfg, p, s, c, item, o)
}

// wakeTarget is a probe wake with its probe row and the probe file as the owner has it.
type wakeTarget struct {
	item    store.Item
	row     store.Probe
	data    []byte
	readErr error
}

// wakeFile finds the probe a wake belongs to and reads its file from the owner's checkout (P-19).
func wakeFile(s *store.Store, c caller, p *project, item string) (t wakeTarget, err error) {
	if t.item, err = addressedItem(s, c, item); err != nil {
		return t, err
	}
	if t.item.Probe == "" {
		return t, refuse("item %s is not a probe wake; answer it with ness reply", item)
	}
	if !t.item.Open() {
		return t, refuse("item %s was already answered (%s)", item, t.item.Disposition)
	}
	if t.row, err = s.Probe(t.item.Probe); err != nil {
		return t, err
	}
	owner, err := s.Session(t.row.Owner)
	if err != nil {
		return t, err
	}
	// From the primary checkout, not p.root: answered from a worktree, p.root is that worktree.
	primary, err := primaryRoot(p)
	if err != nil {
		return t, err
	}
	path := filepath.Join(ownerRoot(primary, owner), filepath.FromSlash(t.row.Path))
	t.data, t.readErr = os.ReadFile(path) //nolint:gosec // the owner's file
	return t, nil
}

func answerVerdict(d Deps, cfg *config.Config, p *project, s *store.Store, c caller, item string, o verdictOpts) error {
	t, err := wakeFile(s, c, p, item)
	if err != nil {
		return err
	}
	row := t.row
	needsEdit := ""
	if o.bug {
		if t.readErr != nil {
			return refuse("%v; a probe bug is flagged against the file's hash", t.readErr)
		}
		needsEdit = probe.Hash(t.data)
	}
	// Post before the item is disposed: a failed post (a missing token) leaves the wake open to retry.
	posted, err := postVerdict(d, cfg, p, s, row, t.data, t.readErr, o)
	if err != nil {
		return err
	}
	now := d.Now()
	if derr := s.Dispose(t.item.ID, store.Resolved, o.state+": "+o.note, now); derr != nil {
		return derr
	}
	if verr := s.RecordVerdict(row.ID, o.state, o.note, needsEdit, now); verr != nil {
		return verr
	}
	label := o.state
	if o.bug {
		label += " (probe bug)"
	}
	say(d.Stdout, "verdict %s recorded for %s", label, probe.Name(row.Path, row.Label))
	if posted != "" {
		say(d.Stdout, "posted to %s", posted)
	}
	return nil
}

// postVerdict posts a judged, non-bug verdict and returns the channel it posted to, or "".
func postVerdict(
	d Deps, cfg *config.Config, p *project, s *store.Store, row store.Probe, data []byte, readErr error, o verdictOpts,
) (string, error) {
	if o.bug {
		return "", nil
	}
	err := readErr
	var pf probe.Probe
	if err == nil {
		pf, err = probe.Parse(row.Path, data)
	}
	if err != nil {
		say(d.Stderr, "ness: cannot read the probe file (%v); the verdict is recorded, nothing is posted", err)
		return "", nil
	}
	if pf.Notify != "judged" {
		return "", nil
	}
	if !cfg.On("notify") {
		say(d.Stderr, "ness: feature notify is off; the verdict is recorded, nothing is posted")
		return "", nil
	}
	_, err = buildPoster(d, cfg, p.root, s, true).Post(notify.Request{
		Channel: pf.Channel, Markdown: probe.VerdictNote(pf, row.Label, o.state, o.note),
		Thread: probe.Thread(row.Path, row.Label),
	})
	return pf.Channel, err
}

// probeFiles turns the arguments into sorted repo-relative paths: a directory means its *.md
// files; a missing path or one outside the checkout refuses before anything is registered.
func probeFiles(root string, args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		files, err := expandProbeArg(a)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			rel, rerr := filepath.Rel(root, f)
			if rerr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, refuse("%s is outside this checkout", f)
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	slices.Sort(out)
	return out, nil
}

// expandProbeArg is the absolute file, or a directory's *.md files; a directory without any refuses.
func expandProbeArg(a string) ([]string, error) {
	abs, err := filepath.Abs(a)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, refuse("%s: %v", a, err)
	}
	if !info.IsDir() {
		return []string{abs}, nil
	}
	files, err := filepath.Glob(filepath.Join(abs, "*.md"))
	if err == nil && len(files) == 0 {
		return nil, refuse("%s has no *.md probe files", a)
	}
	return files, err
}

// oneProbeFile is probeFiles for the commands that act on a single probe: a directory of several
// files is ambiguous and refuses.
func oneProbeFile(root, arg string) ([]string, error) {
	rels, err := probeFiles(root, []string{arg})
	if err == nil && len(rels) != 1 {
		return nil, refuse("%s holds %d probe files; name one", arg, len(rels))
	}
	return rels, err
}

// checkNotify is what a probe's notify policy needs: the notify feature and a configured
// channel that is not a github one, since every probe post is a thread and github posts to
// issues (P-17). The worker's reload applies the same check.
func checkNotify(cfg *config.Config, pf probe.Probe) error {
	if pf.Notify == "none" {
		return nil
	}
	if !cfg.On("notify") {
		return refuse("%s: notify: %s needs the notify feature; feature notify is off: add %q to features in %s",
			pf.Path, pf.Notify, "notify", config.Path)
	}
	ch, ok := cfg.Notify.Channels[pf.Channel]
	if !ok {
		return refuse("%s: channel %s is not in [notify.channels]", pf.Path, pf.Channel)
	}
	if ch.Kind == "github" {
		return refuse("%s: channel %s is a github channel; probes post to threads, github posts to issues",
			pf.Path, pf.Channel)
	}
	return nil
}

// parseProbes reads and validates every file before anything is registered (D15: fail loudly
// at start, not at the first post).
func parseProbes(root string, cfg *config.Config, rels []string, checkChannel bool) ([]probe.Probe, error) {
	out := make([]probe.Probe, 0, len(rels))
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // under the checkout
		if err != nil {
			return nil, refuse("%v", err)
		}
		pf, err := probe.Parse(rel, data)
		if err != nil {
			return nil, refuse("%v", err)
		}
		if nerr := checkNotify(cfg, pf); checkChannel && nerr != nil {
			return nil, nerr
		}
		out = append(out, pf)
	}
	return out, nil
}

// ownerRoot is the checkout a probe's file is read from: the owner's worktree, or the primary
// checkout for an owner without one (P-19).
func ownerRoot(primary string, sess store.Session) string {
	if sess.Worktree == "" {
		return primary
	}
	return filepath.Join(primary, sess.Worktree)
}

func runProbeStart(d Deps, args []string, label string, keep bool) (err error) {
	p, cfg, err := requireFeature(d, "probes")
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	if c.operator() {
		return refuse("a probe needs a session as its owner; the operator owns none (run from a session)")
	}
	rels, err := probeFiles(p.root, args)
	if err != nil {
		return err
	}
	pfs, err := parseProbes(p.root, cfg, rels, true)
	if err != nil {
		return err
	}
	now := d.Now()
	r := rand.New(rand.NewPCG(uint64(now.UnixNano()), 0)) //nolint:gosec // schedule jitter, not a secret
	for _, pf := range pfs {
		row, serr := s.StartProbe(pf.Path, c.id, label, keep, probe.FirstRun(pf, now, r), now)
		if serr != nil {
			return serr
		}
		say(d.Stdout, "started %s owner %s next %s",
			probe.Name(pf.Path, label), row.Owner, row.NextRunAt.UTC().Format(time.RFC3339))
	}
	return nil
}

// runProbeRun is the authoring loop (R-PRB-063): no store, no wake, no post.
func runProbeRun(d Deps, arg string, asJSON bool) error {
	p, cfg, err := requireFeature(d, "probes")
	if err != nil {
		return err
	}
	rels, err := oneProbeFile(p.root, arg)
	if err != nil {
		return err
	}
	pfs, err := parseProbes(p.root, cfg, rels, false)
	if err != nil {
		return err
	}
	pf := pfs[0]
	if pf.Floorless() {
		say(d.Stdout, "no floor\n%s", pf.Scope)
		return nil
	}
	f := probe.Floor{Dir: p.root, Owner: displayName(d.Session), Redact: cfg.Probes.RedactEnv, Getenv: os.Getenv}
	run, err := probe.RunFloor(d.shell(), pf, f)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(map[string]any{
			"schema": 1, "state": run.State, "exit": run.Exit,
			"timed_out": run.TimedOut, "evidence": run.Evidence,
		})
		if err != nil {
			return err
		}
	} else {
		state := run.State
		if run.TimedOut {
			state += " (timeout)"
		}
		say(d.Stdout, "%s\n%s", state, strings.TrimRight(run.Evidence, "\n"))
	}
	if run.State != probe.Green {
		return exitCode(ExitProblems)
	}
	return nil
}

func runProbeStop(d Deps, arg, label, note string) (err error) {
	p, _, err := requireFeature(d, "probes")
	if err != nil {
		return err
	}
	rels, err := oneProbeFile(p.root, arg)
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	if !store.ValidLabel(label) {
		return usage("--label %q: a label matches [A-Za-z0-9][A-Za-z0-9._-]{0,31}", label)
	}
	row, err := stopTarget(s, rels[0], label, c.id)
	if err != nil {
		return err
	}
	if serr := s.SetProbeState(row.ID, store.ProbeStopped, note, d.Now()); serr != nil {
		return serr
	}
	say(d.Stdout, "stopped %s (%s)", probe.Name(rels[0], row.Label), note)
	return nil
}

// stopTarget is the owner's probe a stop means: the one with the label given; with no label and no
// unlabelled probe, the only live one. Several live labels refuse and are listed, so a stop never
// guesses (R-PRB-088).
func stopTarget(s *store.Store, path, label, owner string) (store.Probe, error) {
	row, err := s.ProbeOf(path, owner, label)
	if !errors.Is(err, store.ErrNoProbe) {
		return row, err
	}
	labels, err := s.LiveLabels(path, owner)
	switch {
	case err != nil:
		return row, err
	case len(labels) == 0:
		return row, refuse("no active probe at %s owned by %s", probe.Name(path, label), displayName(owner))
	case label == "" && len(labels) == 1:
		return s.ProbeOf(path, owner, labels[0])
	}
	shown := make([]string, len(labels))
	for i, l := range labels {
		shown[i] = l
		if l == "" {
			shown[i] = "(none)"
		}
	}
	return row, refuse("no active probe at %s; live labels: %s; pass --label",
		probe.Name(path, label), strings.Join(shown, ", "))
}
