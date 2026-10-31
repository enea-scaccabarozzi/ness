package cli

import (
	"errors"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// caller is who runs a command: a session (NESS_SESSION) or the operator (R-SESS-037).
type caller struct {
	id   string        // store.Operator for the operator
	sess store.Session // zero for the operator
}

func (c caller) operator() bool { return c.id == store.Operator }

// root reports whether c is a root session: its manager is the operator (R-MSG-013).
func (c caller) root() bool { return !c.operator() && c.sess.Parent == store.Operator }

// addressed reports whether c may read and answer it: it is addressed to c, or to the operator
// and c sent it (recording the operator's chat answer) or manages its sender, who holds the copy
// (R-MSG-023, R-MSG-024, R-MSG-051). senderParent is the sender's manager, looked up by the caller.
func (c caller) addressed(it store.Item, senderParent string) bool {
	if it.Recipient == c.id {
		return true
	}
	return it.Recipient == store.Operator && !c.operator() && (it.Sender == c.id || senderParent == c.id)
}

// displayName prints a session id, or "operator" for the empty id.
func displayName(id string) string {
	if id == store.Operator {
		return "operator"
	}
	return id
}

// openStore opens the project's store and resolves the caller. A store from a newer ness, and a
// NESS_SESSION the store does not know or has closed, are refusals: the process claims an
// identity it cannot hold.
func openStore(d Deps, p *project) (*store.Store, caller, error) {
	path, err := store.Locate(p.root)
	if err != nil {
		return nil, caller{}, refuse("%v", err)
	}
	s, err := store.Open(path)
	if err != nil {
		return nil, caller{}, refuse("%w", err)
	}
	fillSizes(d, p, s)
	c, err := identify(s, d)
	if err != nil {
		return nil, caller{}, errors.Join(err, s.Close())
	}
	return s, c, nil
}

// fillSizes gives the roots that predate sizes the project's default size, the first time any
// command opens the store, so a project that runs program keeps its program checks on its old
// roots (R-SESS-154). A profile that does not resolve leaves them unsized: sync reports it.
func fillSizes(d Deps, p *project, s *store.Store) {
	if has, err := s.HasUnsizedRoot(); err != nil || !has {
		return
	}
	if _, res, err := resolvedConfig(d, p); err == nil {
		_ = s.FillSizes(res.DefaultSize()) //nolint:errcheck // a busy store fills at the next command
	}
}

// identify resolves the caller: NESS_SESSION when set (unknown or closed ids are refused);
// otherwise the open root adopted by the conversation CLAUDE_CODE_SESSION_ID (R-SESS-114);
// otherwise the operator.
func identify(s *store.Store, d Deps) (caller, error) {
	id := d.Session
	if id == store.Operator {
		sess, err := s.AdoptedBy(d.Conversation)
		if errors.Is(err, store.ErrNoSession) {
			return caller{}, nil
		}
		if err != nil {
			return caller{}, err
		}
		return caller{id: sess.ID, sess: sess}, nil
	}
	sess, err := s.Session(id)
	if errors.Is(err, store.ErrNoSession) {
		return caller{}, refuse("NESS_SESSION=%s is not a session of this project; unset it or run from a ness session",
			id)
	}
	if err != nil {
		return caller{}, err
	}
	if sess.Closed {
		return caller{}, refuse("session %s is closed; unset NESS_SESSION or open a new session", id)
	}
	return caller{id: id, sess: sess}, nil
}

// withStore loads the project, opens the store, identifies the caller and runs fn.
func withStore(d Deps, fn func(s *store.Store, c caller) error) (err error) {
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	return fn(s, c)
}

// addressedItem loads an item and refuses unless it exists and is addressed to c (R-MSG-023).
func addressedItem(s *store.Store, c caller, id string) (store.Item, error) {
	it, err := s.Item(id)
	if errors.Is(err, store.ErrNoItem) {
		return it, refuse("no item %s", id)
	}
	if err != nil {
		return it, err
	}
	senderParent := ""
	if it.Recipient == store.Operator && it.Sender != store.Operator && it.Sender != c.id {
		sender, serr := s.Session(it.Sender)
		if serr != nil && !errors.Is(serr, store.ErrNoSession) {
			return it, serr
		}
		senderParent = sender.Parent
	}
	if !c.addressed(it, senderParent) {
		return it, refuse("item %s is addressed to %s, not to you", id, displayName(it.Recipient))
	}
	return it, nil
}
