package cli

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	pcheck "github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newCloseCmd(d Deps) *cobra.Command {
	var note, keep, authorized string
	var discard, withdraw bool
	cmd := &cobra.Command{
		Use:   "close ID",
		Short: "Close a child you manage, as the operator a root, or your own root with the operator's words",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return withStore(d, func(s *store.Store, c caller) error {
				o := closeOpts{note: note, keep: keep, discard: discard, withdraw: withdraw, authorized: authorized}
				return runClose(d, s, c, args[0], o)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&note, "note", "", "close note `TEXT`, recorded with the close")
	f.StringVar(&authorized, "authorized", "", "close your own root: `WORDS` the operator typed in this conversation")
	f.StringVar(&keep, "keep-work", "", "keep the worktree and branch, recording `REASON`")
	f.BoolVar(&discard, "discard", false, "close without an integration")
	f.BoolVar(&withdraw, "withdraw", false, "withdraw what you sent the session that is still open, the note as reason")
	return cmd
}

type closeOpts struct {
	note, keep, authorized string
	discard, withdraw      bool
}

// withdrawReason is the answer an item withdrawn by close --withdraw carries (R-SESS-130).
func (o closeOpts) withdrawReason() string {
	if o.note == "" {
		return "withdrawn at close"
	}
	return o.note
}

// closable returns the session c may close: the manager closes a child, the operator a root, a
// root itself with the operator's words (checked against the transcript by authorized, R-SESS-121)
// (R-SESS-060).
func closable(s *store.Store, c caller, id string, self bool) (store.Session, error) {
	sess, err := s.Session(id)
	if errors.Is(err, store.ErrNoSession) {
		return sess, refuse("%s is not a session of this project", id)
	}
	if err != nil {
		return sess, err
	}
	if sess.Closed {
		return sess, refuse("session %s is already closed", id)
	}
	return sess, closeRight(sess, c, id, self)
}

// closeRight is the rights rule of closable (R-SESS-060).
func closeRight(sess store.Session, c caller, id string, self bool) error {
	switch {
	case self && (!c.root() || c.id != id):
		return refuse("--authorized closes only your own root; ask your manager to close %s%s",
			id, rule.Ref(rule.RootClose))
	case self:
		return nil
	case sess.Parent == store.Operator && !c.operator():
		return refuse("only the operator closes a root; ask them, then "+
			"`ness close %s --authorized \"their words\"`%s", id, rule.Ref(rule.RootClose))
	case sess.Parent != store.Operator && c.id != sess.Parent:
		return refuse("only %s's manager (%s) closes it%s", id, sess.Parent, rule.Ref(rule.RootClose))
	}
	return nil
}

// closeSpec resolves the project facts a close needs and wires the PR check when a PR is recorded
// or the pr integrator is configured.
func closeSpec(d Deps, sess store.Session, o closeOpts) (session.CloseSpec, string, []profile.Check, error) {
	p, err := loadProject(d)
	if err != nil {
		return session.CloseSpec{}, "", nil, err
	}
	cfg, res, err := resolvedConfig(d, p)
	if err != nil {
		return session.CloseSpec{}, "", nil, err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return session.CloseSpec{}, "", nil, err
	}
	spec := session.CloseSpec{Sess: sess, Primary: primary, Discard: o.discard, Keep: o.keep}
	mode := cfg.Integrate.Mode
	if strings.HasPrefix(sess.Integrated, "http") {
		mode = "pr" // a recorded PR is judged by the pr integrator whatever the mode is now
	}
	if mode == "pr" {
		forge := buildForge(d)
		spec.Merged = func(url string) (bool, error) { return forge.Merged(primary, url) }
	}
	return spec, mode, res.Checks, nil
}

func runClose(d Deps, s *store.Store, c caller, id string, o closeOpts) error {
	sess, err := closable(s, c, id, o.authorized != "")
	if err != nil {
		return err
	}
	spec, mode, checks, err := closeSpec(d, sess, o)
	if err != nil {
		return err
	}
	spec.Withdraw, spec.By = o.withdraw, c.id
	if o.authorized != "" {
		if err := authorized(d, spec.Primary, sess, o.authorized); err != nil {
			return err
		}
	}
	g := session.Git{Exec: d.exec()}
	if err := preClose(d, s, c, g, spec, checks); err != nil {
		return err
	}
	if err := idleRoot(d, sess, o.authorized != ""); err != nil {
		return err
	}
	if err := stopThenLanded(d, g, spec, mode); err != nil {
		return err
	}
	if err := session.Teardown(g, spec); err != nil {
		return err
	}
	if err := finishClose(d, s, g, &spec, c.id, o); err != nil {
		return err
	}
	session.DeleteBranch(g, spec, d.Stderr) // last: a failure above leaves the report findable
	done(d, "closed %s (%s)", id, sess.Scope)
	return nil
}

// preClose is every refusal of a close before any state changes: the landing, then the profile's checks.
func preClose(d Deps, s *store.Store, c caller, g session.Git, spec session.CloseSpec, checks []profile.Check) error {
	if err := session.CheckClose(s, g, spec); err != nil {
		return refuse("%v", err)
	}
	sess := spec.Sess
	if _, err := os.Stat(spec.Checkout()); errors.Is(err, fs.ErrNotExist) {
		if len(checks) > 0 {
			say(d.Stdout, "close checks skipped: the worktree is gone, nothing to examine")
		}
		return nil
	}
	top, err := topRoot(s, sess)
	if err != nil {
		return err
	}
	root, err := s.Session(top)
	if err != nil {
		return err
	}
	return refuseOnChecks(d, s, c, checks, spec.Primary, pcheck.Context{
		Event: "close", Child: sess.Parent != store.Operator, Worktree: spec.Checkout(),
		Branch: sess.Branch, Into: sess.Base, Record: sess.Record, Size: root.Size,
	})
}

// recordContained records the base's sha as the integration of a session that passed CheckClose
// with none recorded, which means its branch is contained in its base (R-SESS-167); stats then see
// it integrated and DeleteBranch drops its branch. It runs after every refusal has passed.
func recordContained(d Deps, s *store.Store, g session.Git, spec *session.CloseSpec) error {
	if spec.Sess.Integrated != "" || spec.Discard || spec.NeverRan() {
		return nil
	}
	sha, err := g.Rev(spec.Primary, spec.Sess.Base)
	if err != nil {
		return err
	}
	if err := s.SetIntegration(spec.Sess.ID, sha, d.Now()); err != nil {
		return err
	}
	spec.Sess.Integrated = sha
	say(d.Stdout, "nothing to integrate: %s is already in %s; recorded %s", spec.Sess.Branch, spec.Sess.Base, sha)
	return nil
}

// moveProbes hands the closing session's probes to its manager, or pauses them when the manager
// is the operator (R-PRB-070, R-PRB-071).
func moveProbes(d Deps, s *store.Store, sess store.Session) error {
	n, err := s.TransferProbes(sess.ID, sess.Parent, d.Now())
	switch {
	case err != nil:
		return err
	case n == 0:
	case sess.Parent == store.Operator:
		say(d.Stdout, "probes: %d paused (no live owner)", n)
	default:
		say(d.Stdout, "probes: %d moved to %s", n, sess.Parent)
	}
	return nil
}

// finishClose records a contained branch's integration (recordContained), withdraws what the
// closer sent the session on --withdraw (R-SESS-130), retires the session's items, settles what it
// sent its manager (R-SESS-123), hands its probes on and records the close. The probes move just
// before the close is recorded: while the owner is still open the worker cannot pause them in
// between (P-08).
func finishClose(d Deps, s *store.Store, g session.Git, spec *session.CloseSpec, by string, o closeOpts) error {
	if err := recordContained(d, s, g, spec); err != nil {
		return err
	}
	sess := spec.Sess
	if o.withdraw {
		n, err := s.SettleFrom(by, sess.ID, store.Withdrawn, o.withdrawReason(), d.Now())
		if err != nil {
			return err
		}
		if n > 0 {
			say(d.Stdout, "withdrew %d item(s) sent to %s", n, sess.ID)
		}
	}
	if _, err := s.Retire(sess.ID, d.Now()); err != nil {
		return err
	}
	if sess.Parent != store.Operator { // the operator's queue is not a manager's inbox
		if _, err := s.SettleFrom(sess.ID, sess.Parent, store.Accepted, "closed with the child", d.Now()); err != nil {
			return err
		}
	}
	if err := moveProbes(d, s, sess); err != nil {
		return err
	}
	return s.RecordClose(sess.ID, o.note, o.keep, d.Now())
}

// idleRoot refuses the operator's close of a root whose agent is working: a root's tab stays
// (R-SESS-122), so nothing stops it before the landed check and a commit made before the worktree
// is removed would be lost (R-SESS-124). A root closing itself is the agent that would commit.
func idleRoot(d Deps, sess store.Session, self bool) error {
	if self || sess.Parent != store.Operator || sess.Pane == "" {
		return nil
	}
	if st, err := buildRuntime(d).State(sess.Pane); err == nil && st.Live && st.Status == "working" {
		return refuse("root %s is working; wait until it is idle, or ask it to close itself", sess.ID)
	}
	return nil
}

// stopThenLanded stops a child's agent first, so nothing can be written between the landed check
// and the teardown, then runs the landed check of a destructive close.
func stopThenLanded(d Deps, g session.Git, spec session.CloseSpec, mode string) error {
	stopped, err := session.CloseTab(buildRuntime(d), spec)
	if err != nil {
		return err
	}
	if !spec.Destructive() {
		return nil
	}
	if err := session.CheckLanded(g, buildIntegrator(d, mode, ""), spec); err != nil {
		if stopped {
			return refuse("%v; the tab is closed and nothing else was touched, "+
				"`ness open --resume %s` relaunches it", err, spec.Sess.ID)
		}
		return refuse("%v; nothing was touched", err)
	}
	return nil
}

// authorizer is a client that can tell whether the operator typed words in conversation id, run
// in dir under home (R-SESS-121).
type authorizer interface {
	OperatorSaid(home, dir, id, words string) (bool, error)
}

// authorized accepts words the operator typed in the root's own transcript (R-SESS-121); anything
// else, and a client that is no authorizer, sends the agent back to ask (R-WRK-092).
func authorized(d Deps, primary string, sess store.Session, words string) error {
	c, ok := buildClient(d, sess.Client).(authorizer)
	if !ok {
		return refuse("client %s cannot check the operator's words; ask the operator to close %s", sess.Client, sess.ID)
	}
	if sess.ClientSession == "" {
		return refuse("no transcript to check %q against; ask the operator to close %s", words, sess.ID)
	}
	said, err := c.OperatorSaid(userHome(), transcriptDir(primary, sess), sess.ClientSession, words)
	if os.IsNotExist(err) {
		return refuse("no transcript of %s yet; ask the operator, then close with their words", sess.ID)
	}
	if err != nil {
		return err
	}
	if !said {
		return refuse("%q is not in a message the operator typed to this session; "+
			"ask them, then close with their words%s", words, rule.Ref(rule.RootClose))
	}
	return nil
}
