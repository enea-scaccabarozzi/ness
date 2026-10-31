package cli

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	pcheck "github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/ladder"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

type openOpts struct {
	scope, tier, brief, title, client, effort, base, resume, size, kind, reason string
	root, here                                                                  bool
}

func newOpenCmd(d Deps) *cobra.Command {
	var o openOpts
	cmd := &cobra.Command{
		Use: "open",
		Example: "  # a root in its own tab and worktree\n" +
			"  ness open --root --scope api --tier standard --size full --brief brief.md\n" +
			"  # a child of this session\n  ness open --scope fix-login --tier economy --brief child.md\n" +
			"  # relaunch an exited session\n  ness open --resume s-0123456789ab",
		Short: "Launch a child or a root session, or resume one",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runOpen(d, o) },
	}
	f := cmd.Flags()
	f.StringVar(&o.scope, "scope", "", "the session's scope `NAME` (required; [a-z][a-z0-9-]{0,31})")
	f.StringVar(&o.tier, "tier", "", "rung from the ladder, or up/down with --resume (default: the selector's)")
	f.StringVar(&o.kind, "kind", "", "work `NAME` for the declared selector (see ness pick)")
	f.StringVar(&o.reason, "reason", "", "why an explicit --tier is above the recommended rung (`TEXT`)")
	f.StringVar(&o.brief, "brief", "", "brief `FILE`, copied to the session's record directory")
	f.StringVar(&o.title, "title", "", "tab title (default: the scope)")
	f.StringVar(&o.client, "client", "", "client for this launch: claude or codex (default: the configured client)")
	f.StringVar(&o.effort, "effort", "", "effort for this launch (default: the tier's)")
	f.StringVar(&o.base, "base", "", "base `REF` of a root (default: the configured base)")
	f.StringVar(&o.size, "size", "", "work size of a root: quick, full or a profile's (default: full)")
	f.StringVar(&o.resume, "resume", "", "relaunch an exited session `ID` or continue its stalled launch")
	f.BoolVar(&o.root, "root", false, "open a root (operator only)")
	f.BoolVar(&o.here, "here", false, "with --root: register this running conversation as the root")
	return cmd
}

// launchEnv is everything an open needs after the project loaded.
type launchEnv struct {
	p       *project
	cfg     *config.Config
	checks  []profile.Check
	sizes   []string
	primary string
	rt      session.Runtime
	client  session.Client
}

// checkFlags refuses what the flags alone get wrong, before the project is read.
func checkFlags(o openOpts) error {
	if o.resume == "" {
		if o.scope == "" {
			return usage("--scope is required")
		}
		if !session.ScopeRe.MatchString(o.scope) {
			return usage("--scope %q: a scope matches [a-z][a-z0-9-]{0,31}", o.scope)
		}
	}
	if o.resume != "" && !session.IDRe.MatchString(o.resume) {
		return refuse("--resume %q is not a session id (s- and 12 lowercase hex characters)", o.resume)
	}
	for _, f := range []struct {
		set       bool
		name, why string
	}{
		{o.here, "--here", "only a root is registered from a running conversation"},
		{o.size != "", "--size", "children have no size"},
		{o.base != "", "--base", "a child starts from its manager's branch"},
	} {
		if f.set && !o.root {
			return refuse("%s needs --root: %s", f.name, f.why)
		}
	}
	return nil
}

func runOpen(d Deps, o openOpts) error {
	if err := checkFlags(o); err != nil {
		return err
	}
	brief, err := readBrief(o.brief)
	if err != nil {
		return err
	}
	return withStore(d, func(s *store.Store, c caller) error {
		if row, rerr := s.Session(o.resume); o.resume != "" && o.client == "" && rerr == nil {
			o.client = row.Client // a relaunch uses the client the session was opened with
		}
		env, err := loadLaunchEnv(d, o)
		if err != nil {
			return err
		}
		if o.resume != "" {
			return runResume(d, s, c, env, o, brief)
		}
		return runLaunch(d, s, c, env, o, brief)
	})
}

func readBrief(arg string) ([]byte, error) {
	if arg == "" {
		return nil, nil
	}
	data, err := os.ReadFile(arg) //nolint:gosec // the caller names its own file
	if err != nil {
		return nil, refuse("--brief %s: %v", arg, err)
	}
	return data, nil
}

func loadLaunchEnv(d Deps, o openOpts) (*launchEnv, error) {
	p, err := loadProject(d)
	if err != nil {
		return nil, err
	}
	cfg, res, err := resolvedConfig(d, p)
	if err != nil {
		return nil, err
	}
	client, err := pickClient(d, cfg, o)
	if err != nil {
		return nil, err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return nil, err
	}
	env := &launchEnv{
		p: p, cfg: cfg, checks: res.Checks, sizes: res.Sizes, primary: primary,
		rt: buildRuntime(d), client: client,
	}
	checks := session.Env{
		Runtime: env.rt, Client: client, Tiers: cfg.Tiers,
		ProjectSkills: filepath.Join(primary, ".agents", "skills"), SkillRoots: skillRoots(),
	}
	if err := checks.Check(); err != nil {
		return nil, refuse("%v", err)
	}
	return env, nil
}

// pickClient builds the client of this launch and validates --client and --effort against it.
func pickClient(d Deps, cfg *config.Config, o openOpts) (session.Client, error) {
	name := cfg.Client
	if o.client != "" {
		name = o.client
	}
	if !slices.Contains(config.Clients, name) {
		return nil, usage("--client %s: accepted values are %s", name, strings.Join(config.Clients, ", "))
	}
	if name == "codex" && !slices.Contains(cfg.Experimental, "codex") {
		return nil, refuse(`the codex client needs experimental = ["codex"] in .ness/ness.toml`)
	}
	client := buildClient(d, name)
	if o.effort != "" && !slices.Contains(client.Efforts(), o.effort) {
		return nil, usage("--effort %s: %s accepts %s", o.effort, name, strings.Join(client.Efforts(), ", "))
	}
	return client, nil
}

// choose picks the rung: the selector's through session.Pick, or the explicit --tier when it is on
// the ladder, under the ceiling and, on a new launch, not above the recommendation without
// --reason (R-MOD-040..043); --effort overrides the rung's for this launch (R-MOD-011).
func choose(d Deps, env *launchEnv, o openOpts, role string, brief []byte) (session.Choice, error) {
	l := ladder.New(env.cfg, env.client.Name())
	b := session.Brief{Text: string(brief), Role: role, Client: env.client.Name(), Kind: o.kind}
	ch, err := session.Pick(buildPolicy(d, env.cfg, env.primary), b, l, env.cfg.Models.RouterThreshold)
	if errors.Is(err, session.ErrNoKind) {
		if o.kind != "" {
			return ch, refuse("--kind %s: %v", o.kind, err)
		}
		ch.Reason = "base: no kind"
	}
	err = nil // a failed selector gives base (R-MOD-042), not a refusal
	if o.tier != "" {
		ch, err = explicit(l, ch, o)
	}
	if o.effort != "" {
		ch.Effort = o.effort
	}
	return ch, err
}

// explicit is the rung --tier names, refused off the ladder, above the ceiling, as up/down
// without --resume, or (on a new launch) above the recommendation rec without --reason.
func explicit(l ladder.Ladder, rec session.Choice, o openOpts) (session.Choice, error) {
	if (o.tier == "up" || o.tier == "down") && o.resume == "" {
		return rec, refuse("--tier %s steps a stored rung: it needs --resume", o.tier)
	}
	i, ok := l.Index(o.tier)
	if !ok || i > l.Ceiling {
		return rec, refuse("--tier %s is not on the ladder up to the ceiling; valid tiers are %s",
			o.tier, strings.Join(rungNames(l.Rungs[:l.Ceiling+1]), ", "))
	}
	if o.resume == "" && i > rec.Index && o.reason == "" {
		return rec, refuse("--tier %s is above the recommended %s for this brief (%s); "+
			"pass --reason TEXT to record why", o.tier, rec.Tier, rec.Reason)
	}
	r := l.Rungs[i]
	ch := session.Choice{Tier: r.Name, Model: r.Model, Effort: r.Effort, Index: i, Confidence: 1, Reason: "explicit"}
	if o.reason != "" {
		ch.Reason += ": " + o.reason
	}
	return ch, nil
}

// rungNames are the names of rs, in order.
func rungNames(rs []ladder.Rung) []string {
	names := make([]string, len(rs))
	for k, r := range rs {
		names[k] = r.Name
	}
	return names
}

func roleOf(root bool) string {
	if root {
		return "root"
	}
	return "child"
}

// manager is the session that will manage the new one: the zero value for a root, the caller
// for a child. Anything else is a refusal before any side effect.
func manager(c caller, root bool) (store.Session, error) {
	switch {
	case root && !c.operator():
		return store.Session{}, refuse("only the operator opens a root; "+
			"a session opens children with ness open --scope%s", rule.Ref(rule.RootOpen))
	case !root && c.operator():
		return store.Session{}, refuse("a child needs a manager: run from a session, or open a root with --root")
	case root:
		return store.Session{}, nil
	}
	return c.sess, nil
}

func checkHere(env *launchEnv) error {
	if os.Getenv("HERDR_PANE_ID") != "" && os.Getenv("HERDR_TAB_ID") != "" && env.client.CurrentSession() != "" {
		return nil
	}
	return refuse("--here needs HERDR_PANE_ID, HERDR_TAB_ID and the client's session id: "+
		"run it from inside the %s session in its herdr pane", env.client.Name())
}

// planLaunch runs every refusal of a new launch before anything is written: who may open it,
// the --here environment, a free scope, the model policy and the brief.
func planLaunch(d Deps, s *store.Store, c caller, env *launchEnv, o openOpts,
	brief []byte,
) (store.Session, session.Choice, error) {
	parent, err := manager(c, o.root)
	if err != nil {
		return parent, session.Choice{}, err
	}
	if herr := checkHereAndSize(env, o, parent); herr != nil {
		return parent, session.Choice{}, herr
	}
	if perr := checkPlace(d, s, parent, o, env); perr != nil {
		return parent, session.Choice{}, perr
	}
	if cerr := openChecks(d, s, c, env, o); cerr != nil {
		return parent, session.Choice{}, cerr
	}
	ch, err := choose(d, env, o, roleOf(o.root), brief)
	if err == nil && brief == nil && !o.here {
		err = refuse("--brief FILE is required for a new session")
	}
	if err == nil {
		err = checkBase(d, env, o, parent)
	}
	return parent, ch, err
}

// checkHereAndSize runs the --here environment check and checkSizes.
func checkHereAndSize(env *launchEnv, o openOpts, parent store.Session) error {
	if o.here {
		if err := checkHere(env); err != nil {
			return err
		}
	}
	return checkSizes(env, o, parent)
}

// checkSizes refuses a root's size the project does not know and a child of a quick root: a
// quick root delegates to subagents, and `ness size full` is the way out (R-SESS-158, R-SESS-159).
func checkSizes(env *launchEnv, o openOpts, parent store.Session) error {
	if o.root {
		return checkSize(o.size, env.sizes)
	}
	if parent.Size == "quick" {
		return refuse("root %s is size quick: a quick root delegates to subagents; "+
			"if the work grew, run `ness size full` first%s", parent.ID, rule.Ref(rule.QuickRoot))
	}
	return nil
}

// openChecks runs the profile's open checks in the opener's checkout, after the manager and --here
// checks and before the first write (R-CHK-001).
func openChecks(d Deps, s *store.Store, c caller, env *launchEnv, o openOpts) error {
	return refuseOnChecks(d, s, c, env.checks, env.primary, pcheck.Context{
		Event: "open", Child: !o.root, Worktree: checkoutOf(env.primary, c.sess), Scope: o.scope, Size: o.size,
	})
}

// checkPlace refuses a scope already open under the same manager, and a child past
// launch.max_children open across the project (R-SESS-152): every child runs its tests on the
// same cores, and past half of them the gates time out. A child parked until a later time holds no
// slot; the refusal lists the children that do, idle from their transcripts as status reads it.
func checkPlace(d Deps, s *store.Store, parent store.Session, o openOpts, env *launchEnv) error {
	siblings, err := s.Children(parent.ID)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(siblings, func(x store.Session) bool { return x.Scope == o.scope }); i >= 0 {
		return refuse("scope %s is already open under %s as %s; "+
			"ness open --resume %s continues it if its launch failed, or choose another --scope",
			o.scope, displayName(parent.ID), siblings[i].ID, siblings[i].ID)
	}
	if o.root {
		return nil
	}
	limit := env.cfg.Launch.MaxChildren
	if limit == 0 {
		limit = max(1, runtime.GOMAXPROCS(0)/2) // the cores this process may use, container limits included
	}
	open, err := s.Sessions(false)
	if err != nil {
		return err
	}
	now := d.Now()
	holders := slices.DeleteFunc(open, func(x store.Session) bool {
		return x.Parent == store.Operator || x.ParkedUntil.After(now)
	})
	if len(holders) >= limit {
		return refuse("%d children are already open in this project (launch.max_children = %d); "+
			"wait for one to land and close it%s%s", len(holders), limit, rule.Ref(rule.ChildCap),
			holderLines(d, holders, env.primary))
	}
	return nil
}

// holderLines lists the children holding a slot, one line each with their idle time from the
// transcript (R-SESS-152); built only for the refusal, since it reads every transcript.
func holderLines(d Deps, holders []store.Session, primary string) string {
	now := d.Now()
	var b strings.Builder
	for _, x := range holders {
		idle := "-"
		if tr, ok := transcriptOf(d, x, primary); ok {
			idle = store.FormatAge(now.Sub(tr.LastActive))
		}
		fmt.Fprintf(&b, "\n  %s %s (manager %s) idle %s", x.ID, x.Scope, x.Parent, idle)
	}
	return b.String()
}

// checkBase refuses a base ref that does not resolve to a commit before the row is written
// (R-CFG-036): the root's --base or configured base, a child's manager branch, the checkout's
// adopted root starts from the same base as a launched one.
func checkBase(d Deps, env *launchEnv, o openOpts, parent store.Session) error {
	ref := parent.Branch
	switch {
	case o.root && o.base != "":
		ref = o.base
	case o.root:
		ref = env.cfg.Base
	}
	if ref == "" || strings.HasPrefix(ref, "-") {
		return refuse("base %q is not a branch or commit: pass --base REF (a detached HEAD has no branch)", ref)
	}
	if _, err := d.exec()(env.primary, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return refuse("base %q does not resolve to a commit in %s; pass an existing --base REF", ref, env.primary)
	}
	return nil
}

func runLaunch(d Deps, s *store.Store, c caller, env *launchEnv, o openOpts, brief []byte) error {
	if o.root && o.size == "" {
		o.size = env.sizes[len(env.sizes)-1] // R-SESS-155: the last size a profile adds, else full
	}
	parent, ch, err := planLaunch(d, s, c, env, o, brief)
	if err != nil {
		return err
	}
	l := newLaunch(env, o, parent, session.NewID(), ch)
	if aerr := s.AddLaunch(l, d.Now()); aerr != nil {
		return aerr
	}
	if o.here {
		return adoptHere(d, s, env, l, brief)
	}
	model, _ := session.TierEffort(env.cfg.Tiers[ch.Tier], env.client.Name())
	err = launcher(d, s, env).Run(session.Request{
		ID: l.ID, Model: model, Effort: ch.Effort,
		FallbackModel: env.cfg.Models.FallbackModel, Ness: executable(), Brief: brief,
	})
	if errors.Is(err, session.ErrStalled) {
		return refuse("session %s did not report ready within %s (stalled launch); "+
			"check its tab, then run ness open --resume %s%s", l.ID, env.cfg.Launch.ReadyTimeout.Duration, l.ID,
			trustHint(s, env, l.ID))
	}
	if err != nil {
		return fmt.Errorf("session %s: %w; fix the cause, then run ness open --resume %s", l.ID, err, l.ID)
	}
	done(d, "opened %s scope %s branch %s record %s tier %s (%s)", l.ID, o.scope, l.Branch, l.Record,
		ch.Tier, ch.Reason)
	return nil
}

// trustHint names the usual cause of a stalled launch when the pane shows the client's trust
// dialog; an unreadable pane or any other screen adds nothing.
func trustHint(s *store.Store, env *launchEnv, id string) string {
	sess, err := s.Session(id)
	if err != nil || sess.Pane == "" {
		return ""
	}
	screen, err := env.rt.ReadScreen(sess.Pane)
	if err != nil || !strings.Contains(strings.ToLower(screen), "trust this folder") {
		return ""
	}
	who := "the client"
	if env.client.Name() == "claude" {
		who = "Claude Code"
	}
	return fmt.Sprintf(". %s is asking whether to trust %s; accept it in the session's tab "+
		"(trusting the project root covers its worktrees), then ness open --resume %s",
		who, filepath.Join(env.primary, sess.Worktree), id)
}

// newLaunch is the registered row of a new session: where its worktree, branch and record dir go
// (R-SESS-020, R-SESS-022).
func newLaunch(env *launchEnv, o openOpts, parent store.Session, id string, ch session.Choice) store.Launch {
	l := store.Launch{
		ID: id, Parent: parent.ID, Scope: o.scope, Title: o.title, Tier: ch.Tier, Effort: ch.Effort,
		Client: env.client.Name(), RouterReason: ch.Reason, Adopted: o.here, Size: o.size,
	}
	if l.Title == "" {
		l.Title = o.scope
	}
	l.Record = path.Join(env.cfg.Sessions, o.scope)
	if !o.root {
		l.Record = path.Join(parent.Record, o.scope)
	}
	suffix := o.scope + "-" + strings.TrimPrefix(id, "s-")
	switch {
	case o.root:
		l.Base = env.cfg.Base
		if o.base != "" {
			l.Base = o.base
		}
		l.Worktree, l.Branch = ".worktrees/"+suffix, "session/"+suffix
	default:
		l.Base, l.Worktree, l.Branch = parent.Branch, ".worktrees/"+suffix, "session/"+suffix
	}
	return l
}

// adoptHere makes the registered root's worktree, branch and brief as a launch does, then binds
// the root to the running conversation instead of a new tab (R-SESS-038, R-SESS-115).
func adoptHere(d Deps, s *store.Store, env *launchEnv, l store.Launch, brief []byte) error {
	if err := launcher(d, s, env).Prepare(l.ID, brief); err != nil {
		return fmt.Errorf("session %s: %w; fix the cause, then run ness close %s --discard and adopt again",
			l.ID, err, l.ID)
	}
	if err := s.BindTab(l.ID, os.Getenv("HERDR_PANE_ID"), os.Getenv("HERDR_TAB_ID"), 1); err != nil {
		return err
	}
	if err := s.BindAgent(l.ID, "", env.client.CurrentSession()); err != nil {
		return err
	}
	done(d, "adopted %s scope %s branch %s worktree %s record %s", l.ID, l.Scope, l.Branch,
		filepath.Join(env.primary, l.Worktree), l.Record)
	say(d.Stdout, "work in that worktree from now on; the primary checkout stays as it is")
	return nil
}

func writeBrief(dir string, brief []byte) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "brief.md"), brief, 0o600)
}

func launcher(d Deps, s *store.Store, env *launchEnv) *session.Launcher {
	l := &session.Launcher{
		Store: s, Runtime: env.rt, Client: env.client, Exec: d.exec(), Primary: env.primary,
		Project: env.cfg.Project, Sessions: env.cfg.Sessions, Home: userHome(), Path: os.Getenv("PATH"),
		Copy: env.cfg.Worktree.Copy, Link: env.cfg.Worktree.Link,
		ReadyTimeout: env.cfg.Launch.ReadyTimeout.Duration, Now: d.Now, Log: d.Stderr,
		SkipPermissions: env.cfg.Launch.Permissions == "skip",
	}
	if env.client.Name() == "claude" {
		l.Read = claude.Read
	}
	return l
}

// lockResume takes the session's resume lock without waiting: two --resume of one session would
// each open a tab. The lock is released by the returned func or when the process ends.
func lockResume(p *project, id string) (func(), error) {
	sp, err := store.Locate(p.root)
	if err != nil {
		return nil, refuse("%v", err)
	}
	lock := filepath.Join(filepath.Dir(sp), "resume-"+id+".lock")
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // beside the store, named by a checked id
	if err != nil {
		return nil, err
	}
	//nolint:gosec // an fd fits an int
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.Join(refuse("session %s is already being resumed by another ness open --resume", id),
			f.Close())
	}
	// The file is never removed: unlinking a lock file races with a second ness that opened it.
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing drops the lock; the file stays
}

// resumable loads the session --resume names and refuses what cannot be resumed.
func resumable(c caller, s *store.Store, id string) (store.Session, error) {
	sess, err := s.Session(id)
	if errors.Is(err, store.ErrNoSession) {
		return sess, refuse("%s is not a session of this project", id)
	}
	if err != nil {
		return sess, err
	}
	switch {
	case sess.Closed:
		return sess, refuse("session %s is closed", sess.ID)
	case !c.operator() && sess.Parent != c.id:
		return sess, refuse("only %s's manager or the operator resumes it", sess.ID)
	case sess.Adopted:
		return sess, refuse("session %s was adopted with --here and has no launch to resume", sess.ID)
	}
	return sess, nil
}

// resumeTier is the tier, effort and recorded reason of a resume, without asking the selector:
// the stored ones (reason ""), the next rung for --tier up or down (staying put at either end,
// and refusing a stored rung off the ladder), or the explicit --tier (R-SESS-051, R-MOD-043);
// --effort overrides the effort alone.
func resumeTier(env *launchEnv, o openOpts, sess store.Session) (tier, effort, reason string, err error) {
	if o.tier == "" {
		return sess.Tier, cmp.Or(o.effort, sess.Effort), "", nil
	}
	l := ladder.New(env.cfg, env.client.Name())
	if o.tier != "up" && o.tier != "down" {
		ch, err := explicit(l, session.Choice{}, o)
		return ch.Tier, cmp.Or(o.effort, ch.Effort), ch.Reason, err
	}
	if i, ok := l.Index(sess.Tier); !ok || i > l.Ceiling {
		return "", "", "", refuse("--tier %s: stored rung %s is not on the ladder up to the ceiling (%s)",
			o.tier, sess.Tier, strings.Join(rungNames(l.Rungs[:l.Ceiling+1]), ", "))
	}
	delta, end := 1, "already at the top"
	if o.tier == "down" {
		delta, end = -1, "already at the bottom"
	}
	r, moved := l.Step(sess.Tier, delta)
	reason = o.tier + " from " + sess.Tier
	if !moved {
		reason = end
	}
	return r.Name, cmp.Or(o.effort, r.Effort), reason, nil
}

// exitedOrStalled reports whether sess already was ready (so it exited and gets a new run) and
// refuses a session that is still running or cannot be relaunched (R-SESS-047). A live client
// still waiting for its first prompt (a trust dialog accepted after the launch stalled) is a
// launch to continue; one that died before it gets a new run (R-SESS-025).
func exitedOrStalled(env *launchEnv, sess store.Session) (bool, error) {
	ready := !sess.ReadyAt.IsZero()
	if ready {
		st, err := env.rt.State(sess.Pane)
		live := err == nil && st.Live
		switch {
		case live && session.PromptPending(sess):
			return false, nil
		case live:
			return ready, refuse("session %s is running; close it or wait for it to exit before --resume", sess.ID)
		case err != nil && !errors.Is(err, session.ErrGone):
			return ready, refuse("session %s: its state could not be read (%v); "+
				"not relaunching a session that may be running",
				sess.ID, err)
		}
	}
	if ready && env.client.NewSessionID() == "" {
		return ready, refuse("%s sessions cannot be resumed: the client has no session id", env.client.Name())
	}
	return ready, nil
}

// runResume continues a stalled launch or relaunches an exited session (R-SESS-045..051).
func runResume(d Deps, s *store.Store, c caller, env *launchEnv, o openOpts, brief []byte) error {
	if _, err := resumable(c, s, o.resume); err != nil { // refuse before any lock file exists
		return err
	}
	unlock, err := lockResume(env.p, o.resume)
	if err != nil {
		return err
	}
	defer unlock()
	sess, err := resumable(c, s, o.resume) // again under the lock: it may have changed meanwhile
	if err != nil {
		return err
	}
	ready, err := exitedOrStalled(env, sess)
	if err != nil {
		return err
	}
	tier, effort, reason, err := resumeTier(env, o, sess)
	if err != nil {
		return err
	}
	if perr := prepareResume(d, s, env, sess, ready, brief, tier, effort, reason); perr != nil {
		return perr
	}
	model, _ := session.TierEffort(env.cfg.Tiers[tier], env.client.Name())
	err = launcher(d, s, env).Run(session.Request{
		ID: sess.ID, Model: model, Effort: effort,
		FallbackModel: env.cfg.Models.FallbackModel, Ness: executable(), Brief: brief,
	})
	if errors.Is(err, session.ErrStalled) {
		return refuse("session %s did not report ready within %s; run ness open --resume %s again",
			sess.ID, env.cfg.Launch.ReadyTimeout.Duration, sess.ID)
	}
	if err != nil {
		return err
	}
	after, err := s.Session(sess.ID)
	if err != nil {
		return err
	}
	done(d, "resumed %s run %s%s%s", sess.ID, strconv.Itoa(after.Run), rungNote(tier, reason),
		oldTab(ready, sess.Tab, after.Tab))
	return nil
}

// rungNote is the done line's " tier T (reason)" when a --tier moved the rung, else "".
func rungNote(tier, reason string) string {
	if reason == "" {
		return ""
	}
	return " tier " + tier + " (" + reason + ")"
}

// oldTab names the exited run's tab a relaunch leaves open beside the new one (R-SESS-048 wants
// a new pane; the old output may be wanted). A stalled launch reopens a tab only when the old
// one is gone.
func oldTab(relaunch bool, before, after string) string {
	if !relaunch || before == "" || before == after {
		return ""
	}
	return " (old tab " + before + " kept)"
}

// prepareResume replaces the brief where the worktree already is (a launch not yet at
// writing-brief writes it itself from the request) and records the tier, the reason a --tier
// moved it (when set) and the new run (R-SESS-049, R-SESS-051, R-MOD-043).
func prepareResume(d Deps, s *store.Store, env *launchEnv, sess store.Session, ready bool,
	brief []byte, tier, effort, reason string,
) error {
	if reason != "" {
		if err := s.SetRouterReason(sess.ID, reason); err != nil {
			return err
		}
	}
	wt := filepath.Join(env.primary, sess.Worktree)
	if _, err := os.Stat(wt); brief != nil && err == nil {
		if werr := writeBrief(filepath.Join(wt, filepath.FromSlash(sess.Record)), brief); werr != nil {
			return werr
		}
	}
	if ready {
		return s.NewRun(sess.ID, "opening-tab", tier, effort, d.Now())
	}
	return s.SetTier(sess.ID, tier, effort)
}
