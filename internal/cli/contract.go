package cli

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newContractCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Freeze files, open one for amendment, or check every frozen file (the gate)",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var reason, item, words string
	freeze := &cobra.Command{
		Use: "freeze PATHS...", Short: "Record each file's blob sha in your contract.lock", Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return withContracts(d, func(e contractEnv) error { return runFreeze(d, e, args) })
		},
	}
	amend := &cobra.Command{
		Use: "amend PATHS...", Short: "Open frozen files for change; the reason goes to decisions.md, one line each",
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if reason == "" {
				return usage("--reason is required: why the frozen file changes")
			}
			return withContracts(d, func(e contractEnv) error { return runAmend(d, e, args, reason, item, words) })
		},
	}
	amend.Flags().StringVar(&reason, "reason", "", "why the file changes (recorded in decisions.md)")
	amend.Flags().StringVar(&item, "item", "", "the item from your manager or the operator approving the change")
	amend.Flags().StringVar(&words, "authorized", "",
		"a project-wide path: `WORDS` the operator typed in this conversation approving the change")
	check := &cobra.Command{
		Use: "check", Short: "Verify every frozen file; exit 1 naming each one that differs", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return withProjectOnly(d, func(e contractEnv) error { return runCheck(d, e) })
		},
	}
	cmd.AddCommand(freeze, amend, check)
	return cmd
}

// contractEnv is what the contract commands share: the project (its root is the caller's
// worktree), the config, the store, the caller and git.
type contractEnv struct {
	p   *project
	cfg *config.Config
	s   *store.Store
	c   caller
	g   session.Git
	x   *session.Extractor // runs the formats' units commands over the merge-base export
}

// baseContracts is the configuration the contract commands and the guard run on (R-CTR-046). The
// merge-base's .ness/ness.toml decides when it has the contracts feature on: the branch cannot
// weaken [contracts], sessions, the feature or the base by editing its own copy. It names the
// same base, or the check refuses. Without it (no file at the merge-base, or the feature off
// there) the branch's own config applies: contracts are adopted on this branch.
func baseContracts(d Deps, p *project, cfg *config.Config) (*config.Config, error) {
	g := session.Git{Exec: d.exec()}
	mb, err := session.MergeBase(g, p.root, cfg.Base)
	if err != nil {
		return cfg, nil //nolint:nilerr // no base to compare with: check reports the merge-base
	}
	data, err := g.Out(p.root, "show", mb+":"+config.Path)
	if err != nil {
		return cfg, nil //nolint:nilerr // no ness.toml at the merge-base
	}
	f, err := config.Parse([]byte(data))
	if err != nil {
		return nil, refuse("%s at %s: %v", config.Path, cfg.Base, err)
	}
	bc, err := projectConfig(d, &project{root: p.root, file: f})
	if err != nil {
		return nil, err
	}
	switch {
	case !bc.On("contracts"):
		return cfg, nil
	case bc.Base != cfg.Base:
		return nil, refuse("base is %q here but %q in %s at the merge-base", cfg.Base, bc.Base, config.Path)
	}
	return bc, nil
}

// requireContracts is requireFeature for the contract commands: see baseContracts.
func requireContracts(d Deps) (*project, *config.Config, error) {
	p, err := loadProject(d)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, nil, err
	}
	if cfg, err = baseContracts(d, p, cfg); err != nil {
		return nil, nil, err
	}
	if !cfg.On("contracts") {
		return nil, nil, refuse("feature contracts is off: add %q to features in %s", "contracts", config.Path)
	}
	return p, cfg, nil
}

func withContracts(d Deps, fn func(contractEnv) error) (err error) {
	p, cfg, err := requireContracts(d)
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	g := session.Git{Exec: d.exec()}
	x := session.NewExtractor(g, p.root, cfg.Base)
	defer func() { err = errors.Join(err, x.Close()) }()
	return fn(contractEnv{p: p, cfg: cfg, s: s, c: c, g: g, x: x})
}

// withProjectOnly is for check: the gate reads the tree and git, and runs the formats' units
// commands in an export of the merge-base. It never opens the store or identifies the caller, so it
// works in a fresh clone and under a stale NESS_SESSION.
func withProjectOnly(d Deps, fn func(contractEnv) error) (err error) {
	p, cfg, err := requireContracts(d)
	if err != nil {
		return err
	}
	g := session.Git{Exec: d.exec()}
	x := session.NewExtractor(g, p.root, cfg.Base)
	defer func() { err = errors.Join(err, x.Close()) }()
	return fn(contractEnv{p: p, cfg: cfg, g: g, x: x})
}

// relPath makes an argument repo-relative (slash form) or refuses a path outside the checkout.
func (e contractEnv) relPath(arg string) (string, error) {
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	// Real paths on both sides, as the guard compares them (R-CTR-012).
	rel, err := filepath.Rel(session.RealFile(e.p.root), session.RealFile(abs))
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", refuse("%s is outside this checkout", arg)
	}
	return filepath.ToSlash(rel), nil
}

// frozen is the branch's session-locked set: every lock under <sessions>. skipped are legacy locks
// identical at the merge-base that do not parse (R-CTR-044).
func (e contractEnv) frozen() (locks []session.Lock, frozen map[string]session.Frozen, skipped []string, err error) {
	mb, err := session.MergeBase(e.g, e.p.root, e.cfg.Base)
	if err != nil {
		return nil, nil, nil, err
	}
	locks, skipped, err = session.FindLocksTolerant(e.g, e.p.root, e.cfg.Sessions, mb)
	if err != nil {
		return nil, nil, nil, err
	}
	return locks, session.FrozenFiles(locks), skipped, nil
}

// project is the project-wide frozen set (globs expanded and moded against the merge-base,
// R-CTR-040, R-CTR-046), held apart from the locks so none hides or opens a path (R-CTR-045).
func (e contractEnv) project() (map[string]session.Frozen, error) {
	mb, err := session.MergeBase(e.g, e.p.root, e.cfg.Base)
	if err != nil {
		return nil, err
	}
	return session.ProjectFrozen(e.g, e.p.root, mb, e.cfg.Contracts, e.x)
}

// projectWide reports whether rel is a path of the merge-base's expansion of the project-wide
// config. It reads the path set only, so no units command runs.
func (e contractEnv) projectWide(rel string) (bool, error) {
	mb, err := session.MergeBase(e.g, e.p.root, e.cfg.Base)
	if err != nil {
		return false, refuse("%v", err)
	}
	names, err := session.ProjectPaths(e.g, e.p.root, mb, e.cfg.Contracts.Frozen)
	if err != nil {
		return false, refuse("%v", err)
	}
	return slices.Contains(names, rel), nil
}

// refuseProject refuses a path the project-wide config covers: freeze never touches it.
func (e contractEnv) refuseProject(rel string) error {
	proj, err := e.projectWide(rel)
	if err == nil && proj {
		return refuse("%s is frozen project-wide ([contracts] frozen); change it on %s", rel, e.cfg.Base)
	}
	return err
}

// lockOwner is the open or closed session whose record directory holds the lock.
func lockOwner(s *store.Store, dir string) (store.Session, bool) {
	all, err := s.Sessions(true)
	if err != nil {
		return store.Session{}, false
	}
	i := slices.IndexFunc(all, func(x store.Session) bool { return x.Record == dir })
	if i < 0 {
		return store.Session{}, false
	}
	return all[i], true
}

// mayTouch refuses unless the caller owns the lock or manages its owner (R-CTR-020).
func (e contractEnv) mayTouch(dir string) error {
	owner, ok := lockOwner(e.s, dir)
	if !ok {
		return refuse("%s/%s belongs to no session of this project", dir, session.LockFile)
	}
	if e.c.id == owner.ID || e.c.id == owner.Parent || e.c.operator() {
		return nil
	}
	return refuse("%s/%s is owned by %s; only it, its manager %s and the operator may change it",
		dir, session.LockFile, owner.ID, displayName(owner.Parent))
}

// existingRels makes every argument repo-relative and refuses, before anything is written, a
// missing path (R-CTR-002) or one already frozen and not amended: only amend reopens a frozen
// file, so the decision is logged (R-CTR-005, R-CTR-007, R-CTR-008).
func (e contractEnv) existingRels(args []string, frozen map[string]session.Frozen) ([]string, error) {
	rels := make([]string, 0, len(args))
	for _, a := range args {
		rel, err := e.relPath(a)
		if err != nil {
			return nil, err
		}
		if _, err = os.Stat(filepath.Join(e.p.root, filepath.FromSlash(rel))); err != nil {
			return nil, refuse("%s does not exist; freeze names existing files", rel)
		}
		if perr := e.refuseProject(rel); perr != nil {
			return nil, perr
		}
		if f, ok := frozen[rel]; ok && f.Sha != session.Amended {
			return nil, refuse("%s is already frozen; run ness contract amend %s --reason TEXT to change it", rel, rel)
		}
		rels = append(rels, rel)
	}
	return rels, nil
}

// runFreeze records each path's blob: an amended path into the lock that holds it (R-CTR-008),
// any other into the caller's own lock (R-CTR-001).
func runFreeze(d Deps, e contractEnv, args []string) error {
	if e.c.operator() {
		return refuse("freeze needs a session: the operator freezes project-wide paths in [contracts] frozen")
	}
	locks, frozen, _, err := e.frozen()
	if err != nil {
		return refuse("%v", err)
	}
	rels, err := e.existingRels(args, frozen)
	if err != nil {
		return err
	}
	return e.writeFrozen(d, locks, frozen, rels)
}

// writeFrozen records the blobs of rels and prints what it froze.
func (e contractEnv) writeFrozen(d Deps, locks []session.Lock, frozen map[string]session.Frozen, rels []string) error {
	byDir := map[string]session.Lock{}
	for _, l := range locks {
		byDir[l.Dir] = l
	}
	own := e.c.sess.Record
	if _, ok := byDir[own]; !ok {
		byDir[own] = session.Lock{Dir: own, Files: map[string]string{}}
	}
	touched := map[string]bool{}
	for _, rel := range rels {
		dir := own
		if f, ok := frozen[rel]; ok { // amended: back into the lock that holds it
			dir = f.LockDir
			if merr := e.mayTouch(dir); merr != nil {
				return merr
			}
		}
		l := byDir[dir]
		if rerr := e.record(d, &l, rel); rerr != nil {
			return rerr
		}
		byDir[dir] = l
		touched[dir] = true
	}
	for _, dir := range slices.Sorted(maps.Keys(touched)) {
		if werr := session.WriteLock(e.p.root, byDir[dir]); werr != nil {
			return werr
		}
	}
	say(d.Stdout, "frozen %s in %s/%s", strings.Join(rels, " "), own, session.LockFile)
	return nil
}

// record freezes rel in l by its kind: a file in a [contracts.formats] format unit by unit
// (R-CTR-049), warning when its units command does not start with a file of the merge-base
// (R-CTR-052), a .md file matching [contracts] prefix as an append-only prefix (R-CTR-033), any
// other .feature file scenario by scenario (R-CTR-031), the rest by blob sha (R-CTR-001). An
// unreadable glob is no match.
func (e contractEnv) record(d Deps, l *session.Lock, rel string) error {
	name, f, ok := e.cfg.Contracts.FormatOf(rel)
	if !ok {
		return e.recordBuiltin(l, rel)
	}
	if !e.x.FromRepo(f) {
		say(d.Stderr, "warning: format %q: the units command %q does not start with a file of the merge-base; "+
			"the extractor comes from the environment", name, f.Units)
	}
	return l.RecordUnits(e.x, e.p.root, rel, name, f)
}

func (e contractEnv) recordBuiltin(l *session.Lock, rel string) error {
	switch session.LockKind(rel, e.cfg.Contracts.Prefix) {
	case session.PrefixTag:
		return l.RecordPrefix(e.p.root, rel)
	case session.ScenarioTag:
		return l.RecordScenarios(e.p.root, rel)
	}
	sha, err := e.g.BlobSha(e.p.root, rel)
	l.Set(rel, sha)
	return err
}

// openAmendment finds the lock holding rel and checks the caller may open it.
func (e contractEnv) openAmendment(rel string) (session.Lock, error) {
	locks, frozen, _, err := e.frozen()
	if err != nil {
		return session.Lock{}, refuse("%v", err)
	}
	f, ok := frozen[rel]
	switch {
	case !ok:
		return session.Lock{}, refuse("%s is not frozen; freeze it first, or edit it freely", rel)
	case f.Sha == session.Amended:
		return session.Lock{}, refuse("%s is already open for amendment", rel)
	}
	if merr := e.mayTouch(f.LockDir); merr != nil {
		return session.Lock{}, merr
	}
	return locks[slices.IndexFunc(locks, func(l session.Lock) bool { return l.Dir == f.LockDir })], nil
}

// appendLine appends one line to the file at name, creating it.
func appendLine(name, line string) (err error) {
	fh, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the session record
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, fh.Close()) }()
	_, err = fh.WriteString(line)
	return err
}

// approved is the amendment approval trail (R-CTR-035). The lock's owner needs an item in the
// store addressed to it from its manager (parent) or the operator, whose body names the file (a
// reply by them is such an item); an item it sent itself, or one about anything else, does not
// count. Anyone else who may touch the lock (its manager, the operator) amends on their own
// authority. This is friction and a record, not a security boundary: the caller is whoever
// NESS_SESSION names, as everywhere in ness. The store is not in the clone, so the gate cannot
// re-verify the item; its id is in decisions.md for the reviewer.
func (e contractEnv) approved(dir, rel, id string) error {
	if owner, ok := lockOwner(e.s, dir); ok && e.c.id != owner.ID {
		return nil
	}
	route := fmt.Sprintf("ask your manager: ness send --to manager --intent request --body \"amend %s: WHY\"; "+
		"when the reply arrives run ness contract amend %s --reason TEXT --item ITEM", rel, rel)
	if id == "" {
		return refuse("amending your own frozen file needs approval; %s", route)
	}
	it, err := e.s.Item(id)
	switch {
	case errors.Is(err, store.ErrNoItem):
		return refuse("no item %s; %s", id, route)
	case err != nil:
		return err
	case it.Recipient != e.c.id || (it.Sender != e.c.sess.Parent && it.Sender != store.Operator):
		return refuse("item %s is not addressed to you from your manager or the operator; %s", id, route)
	case !strings.Contains(it.Body, rel):
		return refuse("item %s does not name %s; ask for approval of this file; %s", id, rel, route)
	}
	return nil
}

// runAmend opens each frozen file and logs the reason, one decisions line per path (R-CTR-005,
// R-CTR-007); a project-wide one per R-CTR-056. Every path is validated before anything is written,
// so one bad path writes nothing.
func runAmend(d Deps, e contractEnv, args []string, reason, item, words string) error {
	var rels []string
	proj := map[string]store.Session{} // project-wide path -> the session whose record logs it
	for _, a := range args {
		rel, err := e.relPath(a)
		if err != nil {
			return err
		}
		if slices.Contains(rels, rel) {
			continue
		}
		sess, pw, err := e.checkAmend(d, rel, item, words)
		if err != nil {
			return err
		}
		if pw {
			proj[rel] = sess
		}
		rels = append(rels, rel)
	}
	if len(proj) > 0 && item != "" {
		say(d.Stderr, "ness: --item ignored for project-wide paths: the operator authorizes them, not an item")
	}
	return writeAmends(d, e, rels, proj, reason, item, words)
}

// writeAmends logs and opens each validated path, in the order named.
func writeAmends(
	d Deps, e contractEnv, rels []string, proj map[string]store.Session, reason, item, words string,
) error {
	for _, rel := range rels {
		var err error
		if sess, ok := proj[rel]; ok {
			err = writeProjectAmend(d, e, sess, rel, reason, words)
		} else {
			err = writeAmend(d, e, rel, reason, item)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// checkAmend validates one path for amend without writing: a project-wide one (an added file too,
// R-CTR-055) returns the session whose record logs it and true; a session-locked one needs its
// approval.
func (e contractEnv) checkAmend(d Deps, rel, item, words string) (store.Session, bool, error) {
	pw, err := e.projectWide(rel)
	if err != nil {
		return store.Session{}, false, err
	}
	if pw || session.MatchAny(e.cfg.Contracts.Frozen, rel) {
		sess, perr := e.projectAmender(d, rel, words)
		return sess, true, perr
	}
	l, err := e.openAmendment(rel)
	if err != nil {
		return store.Session{}, false, err
	}
	return store.Session{}, false, e.approved(l.Dir, rel, item)
}

// writeAmend logs the decision for the session-locked rel and opens it in its lock (re-read, so
// several paths of one lock all stay open).
func writeAmend(d Deps, e contractEnv, rel, reason, item string) error {
	l, err := e.openAmendment(rel)
	if err != nil {
		return err
	}
	if item != "" {
		reason += " [item " + item + "]"
	}
	// The decision is logged first: a failed lock write leaves a logged decision, never an
	// unlogged opening.
	line := fmt.Sprintf("Amend %s: %s (%s)\n", rel, reason, d.Now().UTC().Format("2006-01-02"))
	dec := filepath.Join(e.p.root, filepath.FromSlash(l.Dir), "decisions.md")
	if aerr := appendLine(dec, line); aerr != nil {
		return aerr
	}
	l.Set(rel, session.Amended)
	if werr := session.WriteLock(e.p.root, l); werr != nil {
		return fmt.Errorf("the decision is logged but the lock was not changed: %w", werr)
	}
	say(d.Stdout, "amending %s (%s/%s); freeze it again when done", rel, l.Dir, session.LockFile)
	return nil
}

// projectAmender is the session that may open the project-wide rel on this branch (R-CTR-056): a
// session with the words the operator typed to it (checked as close --authorized checks them,
// R-SESS-121), the checkout's session for the operator on their own authority. It writes nothing.
func (e contractEnv) projectAmender(d Deps, rel, words string) (store.Session, error) {
	if e.c.operator() {
		sess, ok := e.checkoutSession()
		if !ok {
			return store.Session{}, refuse(
				"%s is frozen project-wide and this checkout is no session's; amend it on %s directly", rel, e.cfg.Base)
		}
		return sess, nil
	}
	if words == "" {
		return store.Session{}, refuse("%s is frozen project-wide ([contracts] frozen); ask the operator, then run "+
			"ness contract amend %s --reason TEXT --authorized \"their words\"", rel, rel)
	}
	primary, err := primaryRoot(e.p)
	if err != nil {
		return store.Session{}, err
	}
	if aerr := authorized(d, primary, e.c.sess, words); aerr != nil {
		return store.Session{}, aerr
	}
	return e.c.sess, nil
}

// writeProjectAmend logs the project-wide amendment of rel in sess's decisions.md, which check and
// the guard read; it is a record, not a security boundary.
func writeProjectAmend(d Deps, e contractEnv, sess store.Session, rel, reason, words string) error {
	by, says := "[operator]", "by the operator"
	if words != "" {
		by, says = `[operator: "`+words+`"]`, "by the operator's words"
	}
	line := fmt.Sprintf("Amend %s (project-wide): %s %s (%s)\n", rel, reason, by, d.Now().UTC().Format("2006-01-02"))
	if err := appendLine(filepath.Join(e.p.root, filepath.FromSlash(sess.Record), "decisions.md"), line); err != nil {
		return err
	}
	say(d.Stdout, "amending %s (project-wide, %s); it lands with this branch", rel, says)
	return nil
}

// checkoutSession is the open session whose worktree is this checkout.
func (e contractEnv) checkoutSession() (store.Session, bool) {
	primary, err := primaryRoot(e.p)
	if err != nil {
		return store.Session{}, false
	}
	all, err := e.s.Sessions(false)
	if err != nil {
		return store.Session{}, false
	}
	here := session.RealFile(e.p.root)
	i := slices.IndexFunc(all, func(x store.Session) bool {
		return x.Worktree != "" && session.RealFile(checkoutOf(primary, x)) == here
	})
	if i < 0 {
		return store.Session{}, false
	}
	return all[i], true
}

// runCheck is the gate's check (R-CTR-003, R-CTR-004): read-only, exit 1 naming each problem. It
// fails closed: a malformed lock, a project-wide path absent at base and a frozen path gone from every
// lock since the branch point are problems, not usage errors. It trusts no lock content the branch
// wrote: a frozen file that differs from its reference sha needs a logged amendment (R-CTR-021).
func runCheck(d Deps, e contractEnv) error {
	problems, n, err := checkProblems(d, e)
	if err != nil {
		printErr(d.Stderr, err)
		return exitCode(ExitProblems)
	}
	if problems > 0 {
		return exitCode(ExitProblems)
	}
	say(d.Stdout, "ok\t%d frozen files", n)
	return nil
}

// history is the merge-base with base and every path frozen since (session.HistoricalFrozen).
func (e contractEnv) history(locks []session.Lock) (mb string, hist map[string]session.Frozen, err error) {
	if mb, err = session.MergeBase(e.g, e.p.root, e.cfg.Base); err != nil {
		return "", nil, err
	}
	hist, err = session.HistoricalFrozen(e.g, e.p.root, e.cfg.Sessions, mb, locks)
	return mb, hist, err
}

// printDiffering prints one "differs" line per diff, with what broke for a scenario or prefix lock,
// and returns the paths.
func printDiffering(d Deps, frozen map[string]session.Frozen, diff []session.Diff) []string {
	paths := make([]string, 0, len(diff))
	for _, x := range diff {
		where := ".ness/ness.toml"
		if f := frozen[x.Path]; f.LockDir != "" {
			where = f.LockDir + "/" + session.LockFile
		}
		if x.What != "" {
			where += "\t" + x.What
		}
		say(d.Stdout, "differs\t%s\t%s", x.Path, where)
		paths = append(paths, x.Path)
	}
	return paths
}

// checkProblems prints each problem and returns how many, and how many files are frozen.
func checkProblems(d Deps, e contractEnv) (problems, n int, err error) {
	locks, frozen, skipped, err := e.frozen()
	if err != nil {
		return 0, 0, err
	}
	for _, p := range skipped {
		say(d.Stderr, "warning: %s does not parse (an old harness's lock) but is unchanged from %s: skipped",
			p, e.cfg.Base)
	}
	mb, hist, err := e.history(locks)
	if err != nil {
		return 0, 0, err
	}
	for _, p := range slices.Sorted(maps.Keys(hist)) {
		if _, ok := frozen[p]; !ok { // frozen earlier on this branch, gone from every lock now
			say(d.Stdout, "missing\t%s\t%s/%s", p, hist[p].LockDir, session.LockFile)
			problems++
		}
	}
	skip, nProj, err := e.printDiffs(d, mb, frozen)
	if err != nil {
		return 0, 0, err
	}
	problems += len(skip)
	changed, err := session.Unamended(e.g, e.p.root, mb, frozen, hist, skip)
	if err != nil {
		return 0, 0, err
	}
	for _, p := range changed {
		say(d.Stdout, "changed\t%s\t%s/%s (no Amend line in decisions.md)", p, frozen[p].LockDir, session.LockFile)
		problems++
	}
	return problems, len(frozen) + nProj, nil
}

// printDiffs prints the differing paths of the locks and of the project-wide config and returns
// them (all are reported, whatever Unamended says) and how many project-wide paths are frozen.
func (e contractEnv) printDiffs(
	d Deps, mb string, frozen map[string]session.Frozen,
) (paths []string, nProj int, err error) {
	proj, err := e.project()
	if err != nil {
		return nil, 0, err
	}
	for _, set := range []map[string]session.Frozen{frozen, proj} {
		diff, cerr := session.CheckContracts(e.g, e.p.root, mb, set, e.cfg.Contracts, e.x)
		if cerr != nil {
			return nil, 0, cerr
		}
		var kept []session.Diff
		for _, x := range diff {
			amended := false
			if set[x.Path].LockDir == "" { // project-wide
				if amended, cerr = e.projectApproved(mb, x.Path); cerr != nil {
					return nil, 0, cerr
				}
			}
			if !amended {
				kept = append(kept, x)
			}
		}
		paths = append(paths, printDiffering(d, set, kept)...)
	}
	added, err := e.printAdded(d, mb)
	return append(paths, added...), len(proj), err
}

// projectApproved reports whether the differing or added project-wide path p passes check: its
// content is the one at a tip of base (R-CTR-057), or an amendment for it is logged (R-CTR-056).
// Both of check's project-wide verdicts route through it.
func (e contractEnv) projectApproved(mb, p string) (bool, error) {
	if session.AtBaseTip(e.g, e.p.root, e.cfg.Base, mb, p) {
		return true, nil
	}
	return session.ProjectAmended(e.g, e.p.root, e.cfg.Sessions, mb, p)
}

// printAdded prints one "added" line per file new under a frozen glob and not amended (R-CTR-055)
// and returns the paths.
func (e contractEnv) printAdded(d Deps, mb string) ([]string, error) {
	added, err := session.ProjectAdded(e.g, e.p.root, mb, e.cfg.Contracts.Frozen)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range slices.Sorted(maps.Keys(added)) {
		amended, aerr := e.projectApproved(mb, p)
		if aerr != nil {
			return nil, aerr
		}
		if !amended {
			say(d.Stdout, "added\t%s\t.ness/ness.toml (new under frozen glob %s)", p, added[p])
			paths = append(paths, p)
		}
	}
	return paths, nil
}
