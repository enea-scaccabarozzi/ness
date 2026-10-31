package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/spf13/cobra"

	ness "github.com/enea-scaccabarozzi/ness"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
	"github.com/enea-scaccabarozzi/ness/profiles"
)

type syncOpts struct {
	check, dryRun, asJSON bool
	keep                  []string
}

func newSyncCmd(d Deps) *cobra.Command {
	var o syncOpts
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Render core, features and profiles into this repository",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if o.asJSON && !o.dryRun {
				return &exitError{code: ExitUsage, err: errors.New("--json needs --dry-run")}
			}
			p, err := loadProject(d)
			if err != nil {
				return err
			}
			return runSync(d, p, o)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&o.check, "check", false, "write nothing; exit 1 if anything would change, 3 on conflict")
	f.BoolVar(&o.dryRun, "dry-run", false, "write nothing; print the plan and a diff per changed file")
	f.BoolVar(&o.asJSON, "json", false, "with --dry-run, print the plan as JSON")
	f.StringArrayVar(&o.keep, "keep", nil, "keep your version of conflicted `PATH` until upstream changes again")
	cmd.MarkFlagsMutuallyExclusive("check", "dry-run")
	return cmd
}

func runSync(d Deps, p *project, o syncOpts) (err error) {
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	plan, err := planSync(d, p, root)
	if err != nil {
		return err
	}
	for _, target := range o.keep {
		if kerr := plan.Keep(path.Clean(filepath.ToSlash(target))); kerr != nil {
			return refuse("%v", kerr)
		}
	}
	if !o.check && !o.dryRun {
		if err := applyWithNotes(d, root, plan); err != nil {
			return err
		}
	}
	if err := emit(d, plan, o); err != nil {
		return err
	}
	return syncExit(plan, o.check)
}

// applyWithNotes validates the release notes before Apply so a broken CHANGELOG refuses the
// upgrade whole, then prints them after the write.
func applyWithNotes(d Deps, root *os.Root, plan *nsync.Plan) error {
	notes, err := releaseNotes(d, plan.Previous)
	if err != nil {
		return err
	}
	if err := nsync.Apply(root, plan); err != nil {
		return err
	}
	for _, n := range notes {
		if d.rich() {
			t := d.theme()
			say(d.out(), "\n%s\n%s", t.Badge("ness "+n.Version+" release notes", "accent"), t.Markdown(n.Body))
			continue
		}
		say(d.Stdout, "ness %s release notes\n%s", n.Version, n.Body)
	}
	return nil
}

func emit(d Deps, plan *nsync.Plan, o syncOpts) error {
	if o.asJSON {
		return printPlanJSON(d, plan)
	}
	report(d, plan)
	if o.dryRun {
		printDiffs(d, plan)
	}
	return nil
}

// loadProfiles loads core and the configured profiles, resolved (R-PROF-020).
func loadProfiles(d Deps, p *project, names []string) (*profile.Resolved, error) {
	core, err := profile.LoadCore(profiles.Core(), d.Version)
	if err != nil {
		return nil, err
	}
	src := profile.Source{Builtin: profiles.Builtin(), Version: d.Version}
	if dir := filepath.Join(p.root, ".ness", "profiles"); isDir(dir) {
		src.Local = os.DirFS(dir)
	}
	list := []*profile.Profile{core}
	for _, name := range names {
		pr, lerr := src.Load(name)
		if lerr != nil {
			return nil, refuse("%v", lerr)
		}
		list = append(list, pr)
	}
	res, err := profile.Resolve(list, p.file.Defined)
	if err != nil {
		return nil, refuse("%v", err)
	}
	return res, nil
}

// planSync loads core and the configured profiles, resolves them, checks required tools and
// plans the sync. Every failure here is a refusal: nothing has been written yet.
func planSync(d Deps, p *project, root *os.Root) (*nsync.Plan, error) {
	cfg, res, err := resolvedConfig(d, p)
	if err != nil {
		return nil, err
	}
	if terr := checkTools(res.Tools); terr != nil {
		return nil, terr
	}
	entries, err := nsync.Render(res, cfg, root, profiles.Features())
	if err != nil {
		return nil, refuse("%v", err)
	}
	if rerr := nsync.ResolveRefs(entries, root, append(checkRefs(res.Checks), guardRefs(cfg.Guards)...)); rerr != nil {
		return nil, refuse("%v", rerr)
	}
	plan, err := nsync.Compute(root, entries, nsync.LockMeta{Ness: d.Version, Profiles: res.Versions()})
	if err != nil {
		return nil, refuse("%v", err)
	}
	return plan, nil
}

// checkRefs lists the pointer of each profile-declared check for ResolveRefs.
func checkRefs(checks []profile.Check) []nsync.Ref {
	refs := make([]nsync.Ref, len(checks))
	for i, c := range checks {
		refs[i] = nsync.Ref{Name: "check " + c.ID, Pointer: rule.Pointer{Skill: c.Skill, Section: c.Section}}
	}
	return refs
}

// guardRefs lists the pointer of each [[guards]] entry for ResolveRefs, named guards[N].
func guardRefs(guards []config.Guard) []nsync.Ref {
	refs := make([]nsync.Ref, len(guards))
	for i, g := range guards {
		ptr := rule.Pointer{Skill: g.Skill, Section: g.Section}
		refs[i] = nsync.Ref{Name: fmt.Sprintf("guards[%d]", i), Pointer: ptr}
	}
	return refs
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// checkTools refuses when a profile's required tool is not on PATH (R-SYNC-005).
func checkTools(tools map[string]string) error {
	for _, t := range slices.Sorted(maps.Keys(tools)) {
		if _, err := exec.LookPath(t); err != nil {
			return refuse("profile %s requires %s, which is not on PATH; install it, then run ness sync", tools[t], t)
		}
	}
	return nil
}

// say writes one formatted line; a failed write to the user's terminal has nowhere left to go.
func say(w io.Writer, format string, a ...any) {
	if _, err := fmt.Fprintf(w, format+"\n", a...); err != nil {
		return
	}
}

// report prints one line per changed target and per conflicted path (R-SYNC-022); unreadable
// files go to stderr.
func report(d Deps, plan *nsync.Plan) {
	n := 0
	for _, fp := range plan.Files {
		switch {
		case fp.Err != nil:
			n++ // not in sync: no success line
			say(d.Stderr, "ness: %v; left unchanged", fp.Err)
		case fp.Conflict:
			n++
			reportConflict(d, fp)
		case fp.Kept:
			n++
			mark(d, "=", "dim", "kept %s", fp.Path)
		default:
			for _, s := range fp.Steps {
				n += reportStep(d, s)
			}
		}
	}
	if n == 0 && d.rich() {
		done(d, "in sync: nothing to change")
	}
}

func reportConflict(d Deps, fp nsync.FilePlan) {
	if fp.Link {
		mark(d, "!", "fail", "conflict %s: something else is where ness keeps a symlink; "+
			"move it, or run ness sync --keep %s", fp.Path, fp.Path)
		return
	}
	mark(d, "!", "fail", "conflict %s: merge %s.ness-new into it, then run ness sync --keep %s",
		fp.Path, fp.Path, fp.Path)
}

// reportStep prints one step's line and returns how many it printed.
func reportStep(d Deps, s nsync.Step) int {
	switch s.Action {
	case nsync.ActCreate, nsync.ActAdopt:
		mark(d, "+", "ok", "%s %s", s.Action, s.ID)
	case nsync.ActUpdate:
		mark(d, "~", "warn", "%s %s", s.Action, s.ID)
	case nsync.ActDelete:
		mark(d, "-", "fail", "%s %s", s.Action, s.ID)
	case nsync.ActOrphan:
		mark(d, "?", "dim", "orphaned %s (no longer rendered; it is yours now)", s.ID)
	default:
		return 0
	}
	return 1
}

// mark prints a sync line: plain as is, rich after its glyph (R-UI-025).
func mark(d Deps, glyph, color, format string, a ...any) {
	if !d.rich() {
		say(d.Stdout, format, a...)
		return
	}
	say(d.out(), "%s %s", d.theme().Glyph(glyph, color), fmt.Sprintf(format, a...))
}

func printDiffs(d Deps, plan *nsync.Plan) {
	for _, fp := range plan.Files {
		if fp.Err != nil || (!fp.Conflict && !fp.Writes()) {
			continue
		}
		after, label := string(fp.After), fp.Path
		if fp.Gone {
			after = ""
		}
		if fp.Conflict {
			label += ".ness-new"
		}
		diff := strings.TrimSuffix(udiff.Unified(fp.Path, label, string(fp.Before), after), "\n")
		if d.rich() {
			diff = colorDiff(d.theme(), diff)
		}
		say(d.out(), "%s", diff)
	}
}

// colorDiff colors a unified diff: headers bold, hunks accent, additions green, removals red.
func colorDiff(t ui.Theme, diff string) string {
	lines := strings.Split(diff, "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
			lines[i] = t.Bold.Render(l)
		case strings.HasPrefix(l, "@@"):
			lines[i] = t.Accent.Render(l)
		case strings.HasPrefix(l, "+"):
			lines[i] = t.OK.Render(l)
		case strings.HasPrefix(l, "-"):
			lines[i] = t.Fail.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

type planJSON struct {
	Files  []fileJSON `json:"files"`
	Schema int        `json:"schema"`
}

type fileJSON struct {
	Conflict bool       `json:"conflict"`
	Error    string     `json:"error,omitempty"`
	Path     string     `json:"path"`
	Steps    []stepJSON `json:"steps"`
}

type stepJSON struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

func printPlanJSON(d Deps, plan *nsync.Plan) error {
	out := planJSON{Schema: 1, Files: []fileJSON{}}
	for _, fp := range plan.Files {
		f := fileJSON{Path: fp.Path, Conflict: fp.Conflict, Steps: []stepJSON{}}
		if fp.Err != nil {
			f.Error = fp.Err.Error()
		}
		for _, s := range fp.Steps {
			f.Steps = append(f.Steps, stepJSON{ID: s.ID, Action: string(s.Action)})
		}
		out.Files = append(out.Files, f)
	}
	enc := json.NewEncoder(d.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// syncExit maps a plan to the exit code: 3 on conflict, 1 on an unreadable file or (for
// --check) any change, else 0 (R-SYNC-030, R-CFG-038).
func syncExit(plan *nsync.Plan, check bool) error {
	switch {
	case len(plan.Conflicts()) > 0:
		return exitCode(ExitConflict)
	case len(plan.Errors()) > 0:
		return exitCode(ExitProblems)
	}
	if check {
		changed, err := plan.Changed()
		if err != nil {
			return err
		}
		if changed {
			return exitCode(ExitProblems)
		}
	}
	return nil
}

// releaseNotes returns the release notes of every version after the lock's and up to this
// binary's, oldest first, Running sessions included (R-SYNC-032); none when the versions
// are equal (R-SYNC-033) or there was no lock (a fresh init is not an upgrade).
func releaseNotes(d Deps, previous string) ([]nsync.Note, error) {
	if previous == "" || previous == d.Version {
		return nil, nil
	}
	notes, err := nsync.Notes(cmp.Or(d.Notes, ness.Changelog))
	if err != nil {
		return nil, err
	}
	return nsync.Between(notes, previous, d.Version)
}
