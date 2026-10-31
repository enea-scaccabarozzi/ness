package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

// rich reports that this run draws the rich renderer (R-UI-001).
func (d Deps) rich() bool { return d.ui != nil && d.ui.Rich }

// theme is the rich renderer's palette and helpers; only call it when rich.
func (d Deps) theme() ui.Theme { return d.ui.Theme() }

// out is stdout as the renderer writes it: downsampled to the color profile when rich.
func (d Deps) out() io.Writer {
	if !d.rich() {
		return d.Stdout
	}
	return d.ui.Writer(d.Stdout)
}

// done prints a result line: plain as is, rich after a green check (R-UI-029).
func done(d Deps, format string, a ...any) {
	if !d.rich() {
		say(d.Stdout, format, a...)
		return
	}
	t := d.theme()
	say(d.out(), "%s %s", t.OK.Render(ui.GlyphOK), fmt.Sprintf(format, a...))
}

// reportErr prints a failed run's error: plain `ness: …`, rich a red cross and, for a usage
// error (a bad flag or a usage() refusal), the command's help hint (R-UI-021).
func reportErr(d Deps, root *cobra.Command, args []string, hint bool, err error) {
	if !d.rich() {
		printErr(d.Stderr, err)
		return
	}
	t := d.theme()
	w := d.ui.Writer(d.Stderr)
	say(w, "%s %s", t.Fail.Bold(true).Render(ui.GlyphFail), err)
	if hint {
		cmd, _, ferr := root.Find(args)
		if ferr != nil || cmd == nil {
			cmd = root
		}
		say(w, "  %s", t.Dim.Render("try: "+cmd.CommandPath()+" --help"))
	}
}
