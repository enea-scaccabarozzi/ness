package cli

import (
	"errors"
	"slices"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var dispositions = []string{
	store.Resolved, store.Declined, store.Deferred, store.Accepted, store.Returned, store.Withdrawn,
}

const dispositionList = "resolved | declined | deferred | accepted | returned | withdrawn"

func newReplyCmd(d Deps) *cobra.Command {
	var body, as string
	cmd := &cobra.Command{
		Use:   "reply ITEM",
		Short: "Answer an item and dispose it in one step",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !slices.Contains(dispositions, as) {
				return usage("--as %s: accepted values are %s", as, dispositionList)
			}
			answer, err := readBody(body)
			if err != nil {
				return err
			}
			return withStore(d, func(s *store.Store, c caller) error {
				if as == store.Withdrawn {
					return runWithdraw(d, s, c, args[0], answer)
				}
				return runReply(d, s, c, args[0], answer, as)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "the answer `TEXT`, or @FILE")
	f.StringVar(&as, "as", store.Resolved, dispositionList)
	return cmd
}

// runReply checks the item is open and addressed to the caller (R-MSG-023, R-MSG-025) before the
// store disposes it and delivers the answer (R-MSG-020, R-MSG-022); a reply item is only settled
// (R-MSG-050).
func runReply(d Deps, s *store.Store, c caller, id, answer, as string) error {
	it, err := addressedItem(s, c, id)
	if err != nil {
		return err
	}
	if it.Probe != "" {
		return refuse("item %s is a probe wake: answer it with ness probe verdict %s", id, id)
	}
	if !it.Open() {
		return refuse("item %s is already %s", id, it.Disposition)
	}
	if _, err := s.Reply(id, c.id, answer, as, d.Now()); err != nil {
		switch {
		case errors.Is(err, store.ErrNotRecipient):
			return refuse("item %s is a reply to %s: only its recipient settles it", id, displayName(it.Recipient))
		case errors.Is(err, store.ErrDisposed):
			return refuse("item %s was disposed meanwhile", id)
		}
		return err
	}
	if it.Intent == store.IntentReply {
		done(d, "settled reply %s as %s; nothing was sent back (to go on, ness send)", id, as)
		return nil
	}
	done(d, "replied %s as %s", id, as)
	return nil
}

// runWithdraw settles an open item its caller sent as withdrawn; the reason reaches the recipient
// as a reply item (R-MSG-060, R-MSG-061, R-MSG-062).
func runWithdraw(d Deps, s *store.Store, c caller, id, reason string) error {
	it, err := s.Item(id)
	if errors.Is(err, store.ErrNoItem) {
		return refuse("no item %s", id)
	}
	if err != nil {
		return err
	}
	switch {
	case it.Sender != c.id:
		return refuse("item %s was sent by %s: only its sender withdraws it", id, displayName(it.Sender))
	case it.Intent == store.IntentReply:
		return refuse("item %s is a reply: there is nothing to withdraw", id)
	case it.Probe != "":
		return refuse("item %s is a probe wake: answer it with ness probe verdict %s", id, id)
	case !it.Open():
		return refuse("item %s is already %s", id, it.Disposition)
	}
	if _, err := s.Withdraw(id, reason, d.Now()); err != nil {
		if errors.Is(err, store.ErrDisposed) {
			return refuse("item %s was disposed meanwhile", id)
		}
		return err
	}
	done(d, "withdrew %s", id)
	return nil
}
