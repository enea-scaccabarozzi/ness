package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

type postOpts struct {
	channel, file, thread string
	dryRun                bool
}

// newPostCmd is ness post (R-NTF-001): separate from send because an external post is
// irreversible (R-NTF-006); any session and the operator may run it (R-NTF-008).
func newPostCmd(d Deps) *cobra.Command {
	var o postOpts
	cmd := &cobra.Command{
		Use:   "post",
		Short: "Post one Markdown note to a configured channel, rendered for its platform",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) (err error) {
			if o.channel == "" {
				return usage("--channel is required: a name from [notify.channels]")
			}
			if o.file == "" {
				return usage("--file is required: the Markdown note")
			}
			p, cfg, err := requireFeature(d, "notify")
			if err != nil {
				return err
			}
			req, err := readNote(o, cfg.Notify.Channels[o.channel].Kind)
			if err != nil {
				return err
			}
			s, _, err := openStore(d, p) // p is loaded once
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, s.Close()) }()
			return runPost(d, p, cfg, s, o, req)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.channel, "channel", "", "channel `NAME` from [notify.channels]")
	f.StringVar(&o.file, "file", "", "the Markdown note `FILE`")
	f.StringVar(&o.thread, "thread", "", "thread `KEY`: the first post starts the thread, later ones reply in it")
	f.BoolVar(&o.dryRun, "dry-run", false, "print the exact payload of each part and send nothing")
	return cmd
}

// readNote reads the note and, when note.<channel>.md or note.<kind>.md sits next to it, that
// verbatim text (R-NTF-004). The kind lookup lets note.slack.md serve a slack channel of any name.
func readNote(o postOpts, kind string) (notify.Request, error) {
	data, err := os.ReadFile(o.file) //nolint:gosec // the caller names its own file
	if err != nil {
		return notify.Request{}, usage("--file %s: %v", o.file, err)
	}
	req := notify.Request{Channel: o.channel, Markdown: string(data), Thread: o.thread}
	dir := filepath.Dir(o.file)
	for _, name := range []string{o.channel, kind} {
		if name == "" {
			continue
		}
		beside := filepath.Join(dir, "note."+name+".md")
		if v, verr := os.ReadFile(beside); verr == nil { //nolint:gosec // beside the note
			req.Verbatim = string(v)
			break
		}
	}
	return req, nil
}

func runPost(d Deps, p *project, cfg *config.Config, s *store.Store, o postOpts, req notify.Request) error {
	if o.dryRun {
		req.DryRun = d.Stdout
	}
	n, err := buildPoster(d, cfg, p.root, s, !o.dryRun).Post(req)
	var pe *notify.PartialError
	switch {
	case errors.Is(err, notify.ErrNoChannel):
		return refuse("%v", err)
	case errors.Is(err, store.ErrPostBusy):
		return refuse("%v", err)
	case errors.Is(err, notify.ErrEmpty):
		return usage("%s: the note is empty; nothing to post", o.file)
	case errors.As(err, &pe):
		printErr(d.Stderr, err)
		return exitCode(ExitProblems)
	case err != nil:
		return err
	case o.dryRun:
		return nil
	}
	say(d.Stdout, "posted %s to %s", plural(n, "part"), o.channel)
	return nil
}

// plural is "1 part" or "3 parts".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
