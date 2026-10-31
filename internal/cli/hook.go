package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/hook"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

// unboundNote is what session-start tells a conversation that is not a ness session, in a ness
// project, so the agent can promote it to a root itself (R-HOOK-019).
func unboundNote(sizes []string) string {
	extra := ""
	if def := sizes[len(sizes)-1]; def != "full" {
		extra = fmt.Sprintf("\n%s is the default here; ness-entry, Work sizes, says what each size is.", def)
	}
	return `ness: this conversation is not a ness session yet.
Answer questions here directly. Work that needs a branch (a quick task or a full session) runs
as a session: load the ness-entry skill and promote this conversation with
ness open --root --here --scope NAME --tier TIER --size ` + strings.Join(sizes, "|") + ` --brief FILE
quick: one deliverable, no children, one sitting; full: anything else. You can change it later
with ness size. Propose it in one line first, unless the operator already asked you to start the work.` + extra
}

// noteSizes is the sizes the unbound note offers: the project's, or the core two when its profiles
// do not resolve.
func noteSizes(d Deps, p *project) []string {
	if _, res, err := resolvedConfig(d, p); err == nil {
		return res.Sizes
	}
	return []string{"quick", "full"}
}

// cheatSheet is printed at every start (R-HOOK-006): one line per hot-path command, under 20
// lines, so no session re-reads --help.
const cheatSheet = `ness cheat sheet
  ness inbox                                       list your open items, one line each
  ness inbox --show ITEM...                        read items in full (--unread: every unread one)
  ness inbox --boundary NOTE [--until T]           state written down; compact me; parked until T
  ness send --to manager|ID|operator --body TEXT|@FILE  [--intent request|fyi|decision|handoff|checkpoint] [--urgent]
  ness reply ITEM --body TEXT|@FILE                [--as resolved|declined|deferred|accepted|returned|withdrawn]
  ness open --scope NAME --tier TIER --brief FILE  launch a child
  ness integrate --subject 'type(scope): summary'  fold this branch through the gate into the manager's
  ness close ID [--note TEXT] [--withdraw] [--authorized WORDS]  close a child; your root, with the operator's words
  ness probe start|run|verdict|stop                probes, when the feature is on
  ness pick --kind NAME                            the model and effort for a subagent
Waiting rule: pull once, then end your turn; the worker wakes you when something arrives.`

// newHookCmd is the entry point of the client hooks (R-HOOK-002, R-HOOK-016, R-CFG-053). Outside
// a ness project, without NESS_SESSION, for a session the store does not know, or for an event
// this binary does not know, the result is a silent exit 0: a contributor's client must never see ness fail.
// Two exceptions: session-start for the operator in a ness project prints the unbound note, and
// a skew inside a ness session blocks loudly (R-UPD-020..022).
func newHookCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "hook EVENT",
		Short: "Entry point for client hooks: session-start, pre-tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !slices.ContainsFunc(nsync.HookEvents, func(e nsync.HookEvent) bool { return e.Name == args[0] }) {
				return nil // fail open: an exit 2 in a pre-tool slot would block every tool call
			}
			start := args[0] == "session-start"
			b, st := bind(d, start)
			if st == bindSkew {
				return skewHook(d, b.skew, start)
			}
			if st == bindOperator && start {
				say(d.Stdout, "%s", unboundNote(noteSizes(d, b.p))) // R-HOOK-019
			}
			if st != bindBound {
				return nil
			}
			defer func() { // a close error is logged, never turned into a block (exit 2)
				if cerr := b.s.Close(); cerr != nil {
					printErr(d.Stderr, cerr)
				}
			}()
			if args[0] == "pre-tool" {
				return preTool(d, b)
			}
			return sessionStart(d, b)
		},
	}
}

// bound is a hook's context: the project, the store and the caller's session.
type bound struct {
	p *project
	s *store.Store
	c caller
	// skew is the one line a bindSkew hook prints (R-UPD-020).
	skew string
}

// bindState is the outcome of bind.
type bindState int

const (
	// bindUnbound: the client is not bound here for any reason (no ness project, a broken config
	// or store, an unknown session, no identity at all).
	bindUnbound bindState = iota
	// bindBound: the hook runs for a session; the store is open and the caller closes it.
	bindBound
	// bindOperator: the ness project loaded and the caller is the operator; only bound.p is set
	// and the store is closed.
	bindOperator
	// bindSkew: inside a ness session the project or store cannot run under this binary; only
	// bound.skew is set (R-UPD-020).
	bindSkew
)

// bind resolves the session a hook runs for: NESS_SESSION, or the root adopted by the
// conversation CLAUDE_CODE_SESSION_ID (R-SESS-114). Inside a ness session (inSession) a pin
// mismatch, a newer store or a config that does not parse is bindSkew, checked before the session
// lookup so any session id sees it (R-UPD-020). Any other failure, and any failure outside a
// session, means the client is not bound here and the hook stays silent (R-HOOK-002, R-CFG-053,
// R-UPD-023). An operator in a loaded project is reported apart as
// bindOperator, so a caller can tell that case from every other unbound one. Without any identity
// variable bind returns at once, unless always asks it to load the project anyway (session-start,
// for the unbound note).
// ponytail: every call spawns git rev-parse twice and opens SQLite; cache the store path per
// process or per session if pre-tool latency shows up once the worker adds load (P4).
func bind(d Deps, always bool) (bound, bindState) {
	if !always && d.Session == store.Operator && d.Conversation == "" && !onSessionBranch(d) {
		return bound{}, bindUnbound
	}
	p, err := loadProject(d)
	if err != nil {
		return unboundOrSkew(d, err)
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return unboundOrSkew(d, err)
	}
	if c.operator() {
		if cerr := s.Close(); cerr != nil {
			printErr(d.Stderr, cerr)
		}
		return bound{p: p}, bindOperator
	}
	return bound{p: p, s: s, c: c}, bindBound
}

// unboundOrSkew is bind's answer to a failure: bindSkew for a skew inside a ness session,
// bindUnbound for anything else.
func unboundOrSkew(d Deps, err error) (bound, bindState) {
	var pe *config.PinError
	var ce *config.ParseError
	if (errors.As(err, &pe) || errors.As(err, &ce) || errors.Is(err, store.ErrNewer)) && inSession(d) {
		return bound{skew: skewLine(err)}, bindSkew
	}
	return bound{}, bindUnbound
}

// inSession reports a ness session: NESS_SESSION is set or the branch is a session branch.
func inSession(d Deps) bool { return d.Session != store.Operator || onSessionBranch(d) }

// onSessionBranch reports whether the working directory's branch is session/*: one git call,
// no store read, so an adopted root is a session even when its store does not open (R-UPD-020).
func onSessionBranch(d Deps) bool {
	out, err := d.exec()("", "git", "symbolic-ref", "-q", "--short", "HEAD")
	return err == nil && strings.HasPrefix(string(out), "session/")
}

// skewLine is the one line naming a skew's cause and its fix (R-UPD-020). The pin fix is
// PinError.Fix(), never a second copy (R-CFG-013); anything else not a pin or newer store is the
// config's.
func skewLine(err error) string {
	cause, fix := err.Error(), "correct .ness/ness.toml"
	var pe *config.PinError
	switch {
	case errors.As(err, &pe):
		cause, fix = pe.Error(), pe.Fix()
	case errors.Is(err, store.ErrNewer):
		// store.Open's own fix ("upgrade ness") gives way to the one below.
		cause, fix = strings.TrimSuffix(cause, "; upgrade ness"), "ness self-update (ness doctor shows the rollback)"
	}
	return fmt.Sprintf("hooks cannot run in this session: %s; fix: %s", cause, fix)
}

// skewHook answers a hook under skew: session-start prints the line (R-UPD-022); pre-tool blocks
// with it every call but one plain ness command, and a payload that does not parse (R-UPD-021).
func skewHook(d Deps, skew string, start bool) error {
	if start {
		say(d.Stdout, "ness: %s", skew)
		return nil
	}
	if p, err := hook.Parse(d.Stdin); err == nil && hook.NessCommand(p) {
		return nil
	}
	return block(skew)
}

// block is the client's blocking response: exit 2 with the reason on stderr (R-HOOK-014).
func block(reason string) error { return &exitError{code: ExitUsage, err: errors.New(reason)} }

// preTool runs the guards for a bound session; a payload that does not parse is refused
// (R-HOOK-017). With contracts on, the contract guard refuses an edit of a frozen file
// (R-HOOK-010, R-CTR-009): the file's own worktree decides, never CLAUDE_PROJECT_DIR (R-CTR-010).
func preTool(d Deps, b bound) error {
	p, err := hook.Parse(d.Stdin)
	if err != nil {
		return block(err.Error())
	}
	if reason := hook.PreTool(p); reason != "" {
		return block(reason)
	}
	cfg, err := projectConfig(d, b.p)
	if err != nil { // profiles that do not resolve are a skew inside a session (R-UPD-020)
		if hook.NessCommand(p) {
			return nil
		}
		return block(skewLine(err))
	}
	if serr := spawnGuard(d, b, cfg, p); serr != nil {
		return serr
	}
	if reason := hook.ProjectGuards(p, projectGuards(cfg)); reason != "" {
		return block(reason)
	}
	bc, err := baseContracts(d, b.p, cfg)
	if err != nil {
		return nil //nolint:nilerr // as above: the gate's check reports it
	}
	return contractGuard(d, bc, p)
}

// contractGuard refuses an edit of a frozen file when contracts are on.
func contractGuard(d Deps, cfg *config.Config, p hook.Payload) error {
	files := hook.EditedFiles(p.ToolName, p.ToolInput)
	if len(files) == 0 || !cfg.On("contracts") {
		return nil
	}
	cwd := p.Cwd
	if cwd == "" {
		if wd, werr := os.Getwd(); werr == nil {
			cwd = wd
		}
	}
	gd := session.Guard{
		Git: session.Git{Exec: d.exec()}, Sessions: cfg.Sessions, Warn: d.Stderr,
		Project: cfg.Contracts.Frozen, Prefix: cfg.Contracts.Prefix, Formats: cfg.Contracts, Base: cfg.Base,
	}
	var warn []string
	for _, f := range files {
		reason, soft := gd.Verdict(f, cwd)
		switch {
		case reason == "":
		case !soft:
			return block(reason)
		default:
			warn = append(warn, reason)
		}
	}
	warnContext(d, warn)
	return nil
}

// warnContext lets the edit proceed with warn as added context for the model (R-CTR-036).
func warnContext(d Deps, warn []string) {
	if len(warn) == 0 {
		return
	}
	out, err := json.Marshal(map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "PreToolUse", "additionalContext": strings.Join(warn, "\n"),
	}})
	if err == nil { // strings only: cannot fail
		say(d.Stdout, "%s", out)
	}
}

// projectGuards builds the default guards for cfg's gate, then its [[guards]] in file order
// (R-HOOK-022). Config parse validated every regexp and pointer key.
func projectGuards(cfg *config.Config) []hook.Guard {
	out := hook.DefaultGuards(cfg.Gate)
	for _, g := range cfg.Guards {
		hg := hook.Guard{
			Tool: regexp.MustCompile(g.Tool), Match: regexp.MustCompile(g.Match), Message: g.Message,
			Pointer: rule.Pointer{Skill: g.Skill, Section: g.Section},
		}
		if g.Content != "" {
			hg.Content = regexp.MustCompile(g.Content)
		}
		out = append(out, hg)
	}
	return out
}

// sessionStart runs the shadowing check (R-HOOK-005), writes the ready mark (R-HOOK-003, first
// mark wins, R-HOOK-009; the stored phase is the launch's, never touched here), prints the
// identity with the shown phase (R-HOOK-004), the cheat sheet (R-HOOK-006) and, after a
// compaction, the re-read line and the post-compact files (R-HOOK-007). An adopted root is a session like any other
// (R-HOOK-008).
func sessionStart(d Deps, b bound) error {
	p, err := hook.Parse(d.Stdin)
	if err != nil {
		p = hook.Payload{} // an unreadable payload is a plain startup: identity still loads
	}
	cfg, cerr := projectConfig(d, b.p)
	if cerr != nil { // profiles that do not resolve are a skew, as in preTool (R-UPD-022)
		say(d.Stdout, "ness: %s", skewLine(cerr))
		return nil
	}
	if primary, perr := primaryRoot(b.p); perr == nil {
		serr := session.CheckShadowing(filepath.Join(primary, ".agents", "skills"), skillRoots())
		if errors.Is(serr, session.ErrShadowed) {
			return block(serr.Error())
		}
		if serr != nil { // an unreadable skills dir must not lock the session out
			say(d.Stderr, "ness: %v", serr)
		}
	}
	now := d.Now()
	// A failed ready mark (store busy past busy_timeout) is reported, but the identity still
	// loads: a session without it is worse off than one the worker sees as not yet ready.
	markErr := b.s.MarkReady(b.c.id, now)
	sess, gerr := b.s.Session(b.c.id)
	if gerr != nil {
		sess = b.c.sess
	}
	say(d.Stdout, "%s", identityLine(sess, session.Shown(sess, now, cfg.Launch.ReadyTimeout.Duration)))
	sayRootNotices(d, b.p, sess, cfg)
	sayBehindBase(d, b.p, sess)
	compact := p.Source == "compact"
	if compact {
		say(d.Stdout, "After compaction: re-read your brief and your inbox first.")
	}
	say(d.Stdout, "%s", cheatSheet)
	if compact {
		for _, part := range postCompact(d, b.p) {
			say(d.Stdout, "%s", strings.TrimSuffix(string(part.Text), "\n"))
		}
	}
	return markErr
}

// sayRootNotices prints rootNotices for a root; a child gets none (R-UPD-053).
func sayRootNotices(d Deps, p *project, sess store.Session, cfg *config.Config) {
	if sess.Parent != store.Operator {
		return
	}
	for _, line := range rootNotices(d, p, cfg) {
		say(d.Stdout, "%s", line)
	}
}

// identityLine is the session-start identity; a root's size rides before the phase, a child has
// none (R-HOOK-027).
func identityLine(sess store.Session, phase string) string {
	size := ""
	if sess.Size != "" {
		size = " size " + sess.Size
	}
	return fmt.Sprintf("ness session %s scope %s manager %s%s phase %s",
		sess.ID, sess.Scope, displayName(sess.Parent), size, phase)
}

// postCompact returns the ness-entry#post-compact list slot (R-HOOK-007). A profile error is
// reported, not fatal: sync owns profile validation.
func postCompact(d Deps, p *project) []profile.Part {
	cfg, err := p.file.Config(nil)
	if err != nil {
		say(d.Stderr, "ness: %v", err)
		return nil
	}
	res, err := loadProfiles(d, p, cfg.Profiles)
	if err != nil {
		say(d.Stderr, "ness: %v", err)
		return nil
	}
	return res.Lists["ness-entry#post-compact"]
}
