package ui

import (
	"errors"
	"image/color"
	"io"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/huh/v2"
	"charm.land/huh/v2/spinner"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Glyphs of the rich renderer.
const (
	GlyphOK   = "✓"
	GlyphWarn = "!"
	GlyphFail = "✗"
	GlyphStep = "▸"
	GlyphDot  = "●"
)

// Theme is the rich renderer's palette and the drawing helpers built on it.
type Theme struct {
	Settings
	Accent, OK, Warn, Fail, Dim, Bold, Title lipgloss.Style
	ld                                       lipgloss.LightDarkFunc
}

// palette is a color's light-background and dark-background value; unknown names are dim.
func palette(name string) (light, dark string) {
	switch name {
	case "ok":
		return "#15803D", "#4ADE80"
	case "warn":
		return "#A16207", "#FACC15"
	case "fail":
		return "#B91C1C", "#F87171"
	case "accent":
		return "#4338CA", "#A5B4FC"
	}
	return "#6B7280", "#9CA3AF"
}

// Theme returns the palette for the background (R-UI-011): dark colors on a light terminal and
// light ones on a dark terminal.
func (s Settings) Theme() Theme {
	t := Theme{Settings: s, ld: lipgloss.LightDark(s.Dark), Bold: lipgloss.NewStyle().Bold(true)}
	fg := func(name string) lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Color(name)) }
	t.Accent, t.OK, t.Warn, t.Fail, t.Dim = fg("accent").Bold(true), fg("ok"), fg("warn"), fg("fail"), fg("dim")
	t.Title = t.Accent.Underline(true)
	return t
}

// Color is a palette color by name: ok, warn, fail, accent; anything else is the dim grey.
func (t Theme) Color(name string) color.Color {
	light, dark := palette(name)
	return t.ld(lipgloss.Color(light), lipgloss.Color(dark))
}

// Badge is text in a colored pill.
func (t Theme) Badge(text, colorName string) string {
	return lipgloss.NewStyle().Bold(true).Padding(0, 1).
		Foreground(t.ld(lipgloss.Color("#FFFFFF"), lipgloss.Color("#111827"))).
		Background(t.Color(colorName)).Render(text)
}

// Glyph is g bold in the palette color.
func (t Theme) Glyph(g, colorName string) string {
	return lipgloss.NewStyle().Foreground(t.Color(colorName)).Bold(true).Render(g)
}

// Pill is text in the palette color without a background: the lighter badge for table cells.
func (t Theme) Pill(text, colorName string) string {
	return lipgloss.NewStyle().Foreground(t.Color(colorName)).Render(GlyphDot + " " + text)
}

// Table is a bordered table; the columns listed in right are right-aligned and a last row
// named in total is bold.
type Table struct {
	Headers []string
	Rows    [][]string
	Right   []int
	Total   bool
}

// Render draws the table at most the theme's width wide.
func (t Theme) Render(tb Table) string {
	border := lipgloss.NewStyle().Foreground(t.Color(""))
	out := table.New().Border(lipgloss.RoundedBorder()).BorderStyle(border).
		Headers(tb.Headers...).Rows(tb.Rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			st := lipgloss.NewStyle().Padding(0, 1)
			for _, r := range tb.Right {
				if r == col {
					st = st.Align(lipgloss.Right)
				}
			}
			switch {
			case row == table.HeaderRow:
				return st.Inherit(t.Accent)
			case tb.Total && row == len(tb.Rows)-1:
				return st.Inherit(t.Bold)
			}
			return st
		})
	if w := lipgloss.Width(out.String()); w > t.Width {
		out = out.Width(t.Width)
	}
	return out.String()
}

// Markdown renders md for the terminal, wrapped to the theme's width.
func (t Theme) Markdown(md string) string {
	style := styles.LightStyleConfig
	if t.Dark {
		style = styles.DarkStyleConfig
	}
	r, err := glamour.NewTermRenderer(glamour.WithStyles(style), glamour.WithWordWrap(min(t.Width, 100)-2))
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(out, "\n") + "\n"
}

// Spin runs fn behind a spinner titled title on w when the settings animate, else just runs it.
// A non-nil error means the operator interrupted the spinner: fn may still be running, and its
// results must not be read.
func (t Theme) Spin(w io.Writer, title string, fn func()) error {
	if !t.Animate {
		fn()
		return nil
	}
	var started atomic.Bool
	err := spinner.New().Title(" " + title).WithOutput(w).Action(func() { started.Store(true); fn() }).Run()
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tea.ErrInterrupted), errors.Is(err, huh.ErrUserAborted):
		return err //nolint:wrapcheck // the abort is the caller's to report
	case !started.Load(): // a spinner that could not start still runs the work, once
		fn()
		return nil
	}
	return err //nolint:wrapcheck // the caller reports it
}

// Form is a huh form in the ness palette, line by line when accessible (R-UI-013).
func (t Theme) Form(in io.Reader, out io.Writer, groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).WithAccessible(t.Accessible).
		WithInput(in).WithOutput(out).WithShowHelp(true)
}

// Indent returns a writer that starts every line written to w with prefix.
func Indent(w io.Writer, prefix string) *Indenter {
	return &Indenter{w: w, prefix: prefix, start: true}
}

// Indenter is the writer Indent returns.
type Indenter struct {
	w      io.Writer
	prefix string
	start  bool
}

// MidLine reports that the last write did not end its line.
func (in *Indenter) MidLine() bool { return !in.start }

// Write prefixes each line start in p.
func (in *Indenter) Write(p []byte) (int, error) {
	var b []byte
	for _, c := range p {
		if in.start {
			b = append(b, in.prefix...)
		}
		b = append(b, c)
		in.start = c == '\n'
	}
	if _, err := in.w.Write(b); err != nil {
		return 0, err //nolint:wrapcheck // a writer passes its sink's error on
	}
	return len(p), nil
}
