package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	pcheck "github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

func newIntegrateCmd(d Deps) *cobra.Command {
	var from, into, subject string
	cmd := &cobra.Command{
		Use: "integrate",
		Example: "  # this session's branch into its base as one commit, through the gate\n" +
			"  ness integrate --subject 'feat(cli): add --subject'\n" +
			"  # any pair, as the operator\n  ness integrate --from topic --into main --subject 'fix: x'",
		Short: "Fold a branch into its target through the gate, push if configured, record it",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return withStore(d, func(s *store.Store, c caller) error {
				return runIntegrate(d, s, c, from, into, subject)
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source `BRANCH` (default: this session's branch)")
	cmd.Flags().StringVar(&into, "into", "", "target `BRANCH` (default: this session's base, its manager's branch)")
	cmd.Flags().StringVar(&subject, "subject", "",
		"the squash commit's Conventional Commit `SUBJECT`, type(scope): summary (other modes warn and ignore it)")
	return cmd
}

// resolvePair settles source and target (R-SESS-090, R-SESS-101): a session defaults to its own
// branch and its base; any other pair is for roots and the operator.
func resolvePair(c caller, from, into string) (source, target string, err error) {
	if c.operator() {
		if from == "" || into == "" {
			return "", "", usage("the operator names both --from and --into")
		}
		return from, into, nil
	}
	if from == "" {
		from = c.sess.Branch
	}
	if into == "" {
		into = c.sess.Base
	}
	if !c.root() && (from != c.sess.Branch || into != c.sess.Base) {
		return "", "", refuse("a child integrates only its branch into its manager's; " +
			"--from/--into between other refs is for roots and the operator")
	}
	return from, into, nil
}

// checkPair refuses a source or target that is not a local branch (the fold and the record would
// read some other ref) and a session whose own worktree holds uncommitted work, naming the files
// (R-SESS-103): the brief and report are committed records, so untracked files count.
func checkPair(g session.Git, primary string, c caller, from, into string) error {
	for _, b := range []string{from, into} {
		if _, err := g.Out(primary, "show-ref", "--verify", "--quiet", "refs/heads/"+b); err != nil {
			return refuse("%q is not a local branch; --from and --into take branch names", b)
		}
	}
	if c.operator() || from != c.sess.Branch {
		return nil
	}
	dir := primary
	if c.sess.Worktree != "" {
		dir = filepath.Join(primary, c.sess.Worktree)
	}
	files, err := g.DirtyFiles(dir)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		const shown = 5
		more := ""
		if len(files) > shown {
			more = fmt.Sprintf(" (and %d more)", len(files)-shown)
			files = files[:shown]
		}
		return refuse("the worktree of %s has uncommitted changes; commit them before ness integrate: %s%s%s",
			c.id, strings.Join(files, ", "), more, rule.Ref(rule.DirtyWorktree))
	}
	return nil
}

var conventional = regexp.MustCompile(`^[a-z]+(\([^()]+\))?!?: \S`)

// checkSubject refuses a squash without a Conventional Commit subject (R-SESS-140).
func checkSubject(mode, subject string) error {
	if mode != "squash" {
		return nil
	}
	if !conventional.MatchString(subject) {
		return refuse("a squash lands one commit and needs its subject: --subject 'type(scope): summary', "+
			"a Conventional Commit subject such as 'feat(cli): add --subject' (got %q)%s",
			subject, rule.Ref(rule.Subject))
	}
	return nil
}

// tmpName is the throwaway's name: the session's id, or the source's slug for the operator.
func tmpName(c caller, from string) string {
	if c.operator() {
		return "tmp-op-" + strings.ReplaceAll(from, "/", "-")
	}
	return "tmp-" + c.id
}

// holder names the caller in the target lock a waiting integrate reads (R-SESS-164).
func holder(c caller) string {
	if c.operator() {
		return "the operator"
	}
	return c.id
}

// runIntegrate folds through session.Integrate and records the ref (R-SESS-099).
func runIntegrate(d Deps, s *store.Store, c caller, from, into, subject string) error {
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	cfg, res, err := resolvedConfig(d, p)
	if err != nil {
		return err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return err
	}
	from, into, err = resolvePair(c, from, into)
	if err != nil {
		return err
	}
	g := session.Git{Exec: d.exec()}
	if cerr := checkPair(g, primary, c, from, into); cerr != nil {
		return cerr
	}
	if cfg.Integrate.Mode != "squash" && subject != "" {
		say(d.Stderr, "ness: --subject ignored: integrate.mode %s writes its own message", cfg.Integrate.Mode)
		subject = ""
	}
	if serr := preIntegrate(d, s, c, res.Checks, cfg.Integrate.Mode, subject, pcheck.Context{
		Event: "integrate", Primary: primary, Child: !c.operator() && !c.root(), Worktree: checkoutOf(primary, c.sess),
		Branch: from, Into: into, Record: c.sess.Record,
	}); serr != nil {
		return serr
	}
	return fold(d, s, c, spec{p: p, cfg: cfg, g: g, primary: primary, from: from, into: into, subject: subject})
}

// preIntegrate is the last refusal before the fold: the subject, then the profile's checks.
func preIntegrate(d Deps, s *store.Store, c caller, checks []profile.Check, mode, subject string,
	ctx pcheck.Context,
) error {
	if err := checkSubject(mode, subject); err != nil {
		return err
	}
	return refuseOnChecks(d, s, c, checks, ctx.Primary, ctx)
}

// spec is a resolved integrate: the project, its git and the checked pair.
type spec struct {
	p          *project
	cfg        *config.Config
	g          session.Git
	primary    string
	from, into string
	subject    string
}

// fold runs the integration and records and reports its result.
func fold(d Deps, s *store.Store, c caller, sp spec) error {
	common, err := sp.g.CommonDir(sp.primary)
	if err != nil {
		return err
	}
	lock, err := store.Locate(sp.p.root)
	if err != nil {
		return refuse("%v", err)
	}
	log, start := d.Stderr, d.Now()
	var indented *ui.Indenter
	if d.rich() { // the gate's log under a header (R-UI-029)
		t := d.theme()
		say(d.ui.Writer(d.Stderr), "%s %s %s", t.Accent.Render(ui.GlyphStep+" gate"), sp.cfg.Gate,
			t.Dim.Render("("+sp.from+" into "+sp.into+")"))
		indented = ui.Indent(d.ui.Writer(d.Stderr), t.Dim.Render("│ "))
		log = indented
	}
	res, err := session.Integrate(sp.g, buildIntegrator(d, sp.cfg.Integrate.Mode, sp.subject), session.IntegrateSpec{
		Repo: sp.primary, Common: common, TmpName: tmpName(c, sp.from), LockDir: filepath.Dir(lock), Gate: sp.cfg.Gate,
		Setup:       sp.cfg.Integrate.Setup,
		Integration: session.Integration{Source: sp.from, Target: sp.into}, Push: sp.cfg.Integrate.Push, Log: log,
		Holder: holder(c), Records: sp.cfg.Sessions,
		SSHSet: os.Getenv("GIT_SSH") != "" || os.Getenv("GIT_SSH_COMMAND") != "",
	})
	if indented != nil {
		if indented.MidLine() {
			say(d.Stderr, "")
		}
		took := d.Now().Sub(start).Round(time.Millisecond)
		say(d.ui.Writer(d.Stderr), "%s", d.theme().Dim.Render(fmt.Sprintf("╰ %s", took)))
	}
	if err != nil && !res.MovedRef {
		return integrateFailure(d, err)
	}
	return finish(d, s, c, sp, res, err)
}

// finish records and reports a fold. pushErr is set when the target moved and the push after it
// failed; the ref is then recorded and the error reported, so a retry finds nothing to integrate.
func finish(d Deps, s *store.Store, c caller, sp spec, res session.Result, pushErr error) error {
	// Record only the pair close checks: this session's branch into its own base.
	if !c.operator() && sp.from == c.sess.Branch && sp.into == c.sess.Base {
		if rerr := s.SetIntegration(c.id, res.Ref, d.Now()); rerr != nil {
			return rerr
		}
	}
	if pushErr != nil {
		printErr(d.Stderr, fmt.Errorf("%s moved to %s but the push failed: %w; push it: git push origin %s",
			sp.into, res.Ref, pushErr, sp.into))
		return exitCode(ExitProblems)
	}
	switch {
	case res.Published:
		done(d, "opened %s for %s into %s", res.Ref, sp.from, sp.into)
	case res.MovedRef:
		done(d, "integrated %s into %s at %s", sp.from, sp.into, res.Ref)
	default:
		done(d, "integrated %s into %s (target unchanged, recorded %s)", sp.from, sp.into, res.Ref)
	}
	return nil
}

// integrateFailure maps what Integrate returns to an exit: a failed gate, a conflict or a target
// that moved under the gate is a problem (1), a precondition that does not hold is a refusal (4).
func integrateFailure(d Deps, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, session.ErrGate), errors.Is(err, session.ErrTargetMoved):
		printErr(d.Stderr, err)
		return exitCode(ExitProblems)
	case errors.Is(err, session.ErrConflict):
		printErr(d.Stderr, fmt.Errorf("%w; resolve in the session branch, then integrate again", err))
		return exitCode(ExitProblems)
	case errors.Is(err, session.ErrNoGate), errors.Is(err, session.ErrNothingToIntegrate),
		errors.Is(err, session.ErrDirtyTarget):
		return refuse("%v", err)
	}
	return err
}
