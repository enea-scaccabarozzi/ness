package cli

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// sendIntents are the intents a send may carry; reply items come only from ness reply (R-MSG-049).
var sendIntents = []string{
	store.IntentRequest, store.IntentFYI, store.IntentDecision, store.IntentHandoff, store.IntentCheckpoint,
}

type sendOpts struct {
	to, body, intent string
	page             string
	urgent           bool
}

func newSendCmd(d Deps) *cobra.Command {
	var o sendOpts
	cmd := &cobra.Command{
		Use: "send",
		Example: "  ness send --to manager --body 'done, see report.md' --intent handoff\n" +
			"  ness send --to operator --body @question.md --intent decision --urgent\n" +
			"  ness send --to operator --body @progress.md --intent checkpoint --page https://example.test/p",
		Short: "Send a message to your manager, one of your children, another root (as a root) or the operator",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if o.to == "" {
				return usage("--to is required: a session id, manager or operator")
			}
			if !slices.Contains(sendIntents, o.intent) {
				return usage("--intent %s: accepted values are request, fyi, decision, handoff, checkpoint", o.intent)
			}
			if o.page != "" && o.intent != store.IntentCheckpoint {
				return usage("--page goes with --intent checkpoint only")
			}
			body, err := readBody(o.body)
			if err != nil {
				return err
			}
			return withStore(d, func(s *store.Store, c caller) error {
				return runSend(d, s, c, o, body)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.to, "to", "", "recipient: a session `ID`, manager or operator")
	f.StringVar(&o.body, "body", "", "the message `TEXT`, or @FILE to read it from a file")
	f.StringVar(&o.intent, "intent", store.IntentRequest, "request | fyi | decision | handoff | checkpoint")
	f.StringVar(&o.page, "page", "", "a page `URL` for a checkpoint; appended as a Page: line")
	f.BoolVar(&o.urgent, "urgent", false, "ask the worker to hint the recipient at its next pass")
	return cmd
}

func runSend(d Deps, s *store.Store, c caller, o sendOpts, body string) error {
	to, peer, err := resolveRecipient(s, c, o.to)
	if err != nil {
		return err
	}
	body, err = checkpointBody(c, o, to, body)
	if err != nil {
		return err
	}
	id, created, err := s.Send(c.id, to, o.intent, body, o.urgent, d.Now())
	if err != nil {
		return err
	}
	// idempotent too: a repeat after a failed copy makes it
	if to == store.Operator && !c.root() {
		note := CopyHeader(id) + "\n\n" + body
		if _, _, err = s.Send(c.id, c.sess.Parent, store.IntentFYI, note, false, d.Now()); err != nil {
			return err
		}
	}
	if peer {
		note := PeerCopyHeader(id, c.id, to) + "\n\n" + body
		if _, _, err = s.Send(c.id, store.Operator, store.IntentFYI, note, false, d.Now()); err != nil {
			return err
		}
	}
	if !created {
		done(d, "already sent %s", id)
		return nil
	}
	done(d, "sent %s to %s", id, displayName(to))
	return nil
}

// checkpointBody checks a handoff's headings, then a checkpoint's sender, recipient and headings,
// and appends its Page line (R-MSG-065, R-MSG-066, R-MSG-070); any other intent passes through.
func checkpointBody(c caller, o sendOpts, to, body string) (string, error) {
	if o.intent == store.IntentHandoff {
		if missing := store.CheckpointMissing(body); len(missing) > 0 {
			return "", usage("a handoff needs the headings: ## %s%s",
				strings.Join(missing, ", ## "), rule.Ref(rule.Handoff))
		}
	}
	if o.intent != store.IntentCheckpoint {
		return body, nil
	}
	if !c.root() || to != store.Operator {
		return "", refuse("only a root sends a checkpoint, and only to the operator%s", rule.Ref(rule.Checkpoint))
	}
	if missing := store.CheckpointMissing(body); len(missing) > 0 {
		return "", usage("a checkpoint needs the headings: ## %s%s",
			strings.Join(missing, ", ## "), rule.Ref(rule.Checkpoint))
	}
	if o.page != "" && store.PageOf(body) != "" {
		return "", usage("--page %s: the body already has a Page: line", o.page)
	}
	if o.page != "" {
		body = strings.TrimRight(body, "\n") + "\n\nPage: " + o.page
	}
	return body, nil
}

// CopyHeader is the first line of the fyi a child's operator item sends its manager (R-MSG-051),
// so the manager's map stays current without a second question to answer.
func CopyHeader(id string) string { return "[copy of " + id + " to operator]" }

// PeerCopyHeader is the first line of the fyi a root-to-root send files with the operator
// (R-MSG-073), so the operator sees what the roots of a project tell each other.
func PeerCopyHeader(id, from, to string) string {
	return "[copy of " + id + " from " + from + " to " + to + "]"
}

// readBody returns TEXT literally, or the bytes of FILE for @FILE (R-MSG-012). An empty body is
// a usage error: there is nothing to deliver.
func readBody(arg string) (string, error) {
	body := arg
	if strings.HasPrefix(arg, "@") {
		data, err := os.ReadFile(arg[1:]) //nolint:gosec // the caller names its own file
		if err != nil {
			return "", usage("--body %s: %v", arg, err)
		}
		body = string(data)
	}
	if body == "" {
		return "", usage("--body is empty; give the message text, or @FILE")
	}
	return body, nil
}

// resolveRecipient applies the routing rules (R-MSG-013, R-MSG-017, R-MSG-018): a session talks
// to its manager and its children, any session addresses the operator for what its manager
// cannot settle (CX-17), a root addresses the other roots of its project (R-MSG-073), and the
// operator (no session) addresses roots. It returns the recipient id, store.Operator for the
// operator, and peer when a root addresses another root.
func resolveRecipient(s *store.Store, c caller, to string) (id string, peer bool, err error) {
	switch to {
	case "operator":
		if c.operator() {
			return "", false, refuse("the operator cannot send to the operator; name a root session id with --to")
		}
		return store.Operator, false, nil
	case "manager":
		if c.operator() {
			return "", false, refuse("the operator has no manager; name a session id with --to")
		}
		if c.root() {
			return store.Operator, false, nil
		}
		to = c.sess.Parent // a manager goes through the same closed-session check as any id
	}
	target, err := s.Session(to)
	if errors.Is(err, store.ErrNoSession) {
		return "", false, refuse("%s is not a session of this project", to)
	}
	if err != nil {
		return "", false, err
	}
	if target.Closed {
		return "", false, refuse("session %s is closed", to)
	}
	if msg := routeRefusal(c, target); msg != "" {
		return "", false, refuse("%s", msg)
	}
	return to, peerRoot(c, target), nil
}

// routeRefusal is why c may not address target, or "" when it may.
func routeRefusal(c caller, target store.Session) string {
	switch {
	case c.operator() && target.Parent == store.Operator:
		return ""
	case c.operator():
		return target.ID + " is not a root; the operator talks to roots"
	case target.ID == c.sess.Parent, target.Parent == c.id:
		return ""
	case peerRoot(c, target):
		return ""
	default:
		return target.ID + " is neither your manager nor one of your children; " +
			"siblings talk through the manager; only roots address other roots"
	}
}

// peerRoot reports whether c is a root addressing another root, which the operator gets a copy of
// (R-MSG-073).
func peerRoot(c caller, target store.Session) bool {
	return c.root() && target.Parent == store.Operator && target.ID != c.id
}
