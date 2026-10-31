package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// checkResize holds the preconditions: growing has none, but a root cannot leave a size its
// profile declared (the checks gated to it would stop running), which is the operator's call, and
// shrinking to quick needs no open child, because a quick root has none.
func checkResize(s *store.Store, c caller, root store.Session, size string) error {
	if !c.operator() && root.Size != "quick" && root.Size != "full" && root.Size != size {
		return refuse("only the operator moves a root off size %s: ask them to run `ness size %s --session %s`",
			root.Size, size, root.ID)
	}
	if size != "quick" || root.Size == "quick" {
		return nil
	}
	kids, err := s.Children(root.ID)
	if err != nil {
		return err
	}
	if len(kids) > 0 {
		return refuse("close or finish %s first: a quick root has no children%s", kids[0].ID, rule.Ref(rule.QuickRoot))
	}
	return nil
}

func newSizeCmd(d Deps) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use: "size [SIZE]",
		Example: "  # the current size and the valid ones\n  ness size\n" +
			"  # the work grew: a quick root becomes a full session\n  ness size full\n" +
			"  # the operator resizes a root\n  ness size full --session s-0123456789ab",
		Short: "Show or change the work size of a root",
		Args:  cobra.MaximumNArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return runSize(d, args, id) },
	}
	cmd.Flags().StringVar(&id, "session", "", "the root `ID` to show or resize (the operator names it)")
	return cmd
}

// sizeObligations is the one pointer line printed when a root grows into a size: what the size
// asks of it. A profile size without an entry points at the size list.
var sizeObligations = map[string]string{
	"quick":   "no children, no specs.md or plan.md; report.md and the gate as always (ness-entry ## Quick root)",
	"full":    "write specs.md and plan.md before the next commit (ness-entry ## Full session)",
	"program": "load ness-program; its open checks apply from your next child",
}

// checkSize refuses a size the project does not know, naming the ones it does (R-SESS-158).
func checkSize(size string, sizes []string) error {
	for _, s := range sizes {
		if s == size {
			return nil
		}
	}
	return refuse("size %q is not one of: %s%s", size, strings.Join(sizes, ", "), rule.Ref(rule.Size))
}

// sizeTarget is the root `ness size` acts on: the caller's own, or the one the operator names.
func sizeTarget(s *store.Store, c caller, id string) (store.Session, error) {
	if !c.operator() {
		if !c.root() {
			return store.Session{}, refuse("children have no size; ask your root")
		}
		if id != "" && id != c.id {
			return store.Session{}, refuse("a root shows and changes only its own size")
		}
		return s.Session(c.id)
	}
	if id == "" {
		return store.Session{}, refuse("name the root with --session ID")
	}
	root, err := s.Session(id)
	switch {
	case errors.Is(err, store.ErrNoSession):
		return root, refuse("%s is not a session of this project", id)
	case err != nil:
		return root, err
	case root.Parent != store.Operator:
		return root, refuse("children have no size; %s is a child", id)
	case root.Closed:
		return root, refuse("session %s is closed", id)
	}
	return root, nil
}

func runSize(d Deps, args []string, id string) error {
	return withStore(d, func(s *store.Store, c caller) error {
		root, err := sizeTarget(s, c, id)
		if err != nil {
			return err
		}
		p, err := loadProject(d)
		if err != nil {
			return err
		}
		_, res, err := resolvedConfig(d, p)
		if err != nil {
			return err
		}
		if len(args) == 0 {
			say(d.Stdout, "size %s (%s)", root.Size, strings.Join(res.Sizes, ", "))
			return nil
		}
		return resize(d, s, c, root, args[0], res.Sizes)
	})
}

// resize sets the root's size (R-SESS-160).
func resize(d Deps, s *store.Store, c caller, root store.Session, size string, sizes []string) error {
	if err := checkSize(size, sizes); err != nil {
		return err
	}
	if err := checkResize(s, c, root, size); err != nil {
		return err
	}
	if err := s.SetSize(root.ID, size); err != nil {
		return err
	}
	text, ok := sizeObligations[size]
	if !ok {
		text = "see ness-entry ## Work sizes"
	}
	done(d, "size %s: %s", size, text)
	return nil
}
