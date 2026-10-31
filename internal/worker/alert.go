package worker

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// alertRunes is the longest alert text, the "…" of a cut aside (R-WRK-082).
const alertRunes = 400

// AlertKind is the kind of an operator alert (R-WRK-082).
type AlertKind string

// The four kinds of operator alert.
const (
	AlertNeedsYou   AlertKind = "needs-you"
	AlertCheckpoint AlertKind = "checkpoint"
	AlertTrouble    AlertKind = "trouble"
	AlertYourMove   AlertKind = "your-move"
)

// Alert is one message to the operator's channel. Title is the session's title; Session is the URL
// that opens the session in the client's app, "" when none is known; Text is Markdown of at most 400
// runes plus the "…" of a cut, its lines kept; Page is "" without a link; Root is the id of the root
// of the session's tree, the thread key (R-WRK-082).
type Alert struct {
	Kind                                AlertKind
	Project, Title, Session, Text, Page string
	Root                                string
}

var kindEmoji = map[AlertKind]string{
	AlertNeedsYou: "🟠", AlertCheckpoint: "🟢", AlertTrouble: "🔴", AlertYourMove: "🟡",
}

var kindWords = map[AlertKind]string{
	AlertNeedsYou: "Needs you", AlertCheckpoint: "Checkpoint", AlertTrouble: "Trouble", AlertYourMove: "Your move",
}

// Markdown is the note the poster renders for the alert (R-WRK-082): a header of the kind and the
// project over the bold title (a link to the session when known), the text with its bare URLs named,
// and the page link. The project and title are escaped so they render as written; a page that is
// not a safe http(s) URL is plain text.
func (a Alert) Markdown() string {
	title := mdText(a.Title)
	if u, ok := linkURL(a.Session); ok {
		title = "[" + title + "](" + u + ")"
	}
	md := fmt.Sprintf("%s **%s** · %s\n**%s**", kindEmoji[a.Kind], kindWords[a.Kind], mdText(a.Project), title)
	if a.Text != "" {
		md += "\n\n" + listBreaks(linkify(a.Text))
	}
	if u, ok := linkURL(a.Page); ok {
		md += "\n\n[📄 Open the page](" + u + ")"
	} else if a.Page != "" {
		md += "\n\n📄 " + mdText(a.Page)
	}
	return md
}

// mdText escapes the characters inline Markdown reads, so s renders as written.
var mdText = strings.NewReplacer(`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`).Replace

// parens percent-encodes the parentheses a Markdown link target cannot hold.
var parens = strings.NewReplacer("(", "%28", ")", "%29")

// linkURL is raw as a Markdown link target: an http(s) URL with a host and none of the characters
// that end a target or open markup, its parentheses percent-encoded; false for anything else.
func linkURL(raw string) (string, bool) {
	u := parsedLink(raw)
	return parens.Replace(raw), u != nil
}

// parsedLink is raw parsed when linkURL accepts it, else nil.
func parsedLink(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" ||
		strings.ContainsAny(raw, " \t\n<>[]*\"`") {
		return nil
	}
	return u
}

// spanRe matches the Markdown a cut must not split and linkify must look into: a link, an
// autolink, a code span, bold, italic, and a bare http(s) URL (without trailing punctuation).
// ponytail: a URL with parentheses or brackets in it ends before them.
var spanRe = regexp.MustCompile(`\[[^\]\n]*\]\([^)\s]*\)|<https?://[^>\s]+>|` + "`[^`\n]*`" +
	`|\*\*[^\n]+?\*\*|__[^\n]+?__|\*[^*\s](?:[^*\n]*[^*\s])?\*|` + bareURL)

// bareURL ends before "*", a backtick and the "…" of a cut, so neither lands in a link target.
const bareURL = `https?://[^\s<>()\[\]*…` + "`" + `]*[^\s<>()\[\]*….,;:!?'"` + "`" + `]`

// linkify names every bare or angle-bracketed http(s) URL of text, inside bold and italic too:
// `[📄 page](url)` for a claude.ai artifact, `[🔗 host](url)` for any other. Links and code stay.
func linkify(text string) string {
	return spanRe.ReplaceAllStringFunc(text, func(m string) string {
		switch {
		case strings.HasPrefix(m, "http"):
			return named(m)
		case strings.HasPrefix(m, "<http"):
			return named(m[1 : len(m)-1])
		case strings.HasPrefix(m, "**"), strings.HasPrefix(m, "__"):
			return m[:2] + linkify(m[2:len(m)-2]) + m[:2]
		case strings.HasPrefix(m, "*"):
			return "*" + linkify(m[1:len(m)-1]) + "*"
		}
		return m
	})
}

// named is the Markdown link of a bare URL, labeled by what it opens; a URL linkURL refuses stays text.
func named(raw string) string {
	u := parsedLink(raw)
	if u == nil {
		return raw
	}
	target := parens.Replace(raw)
	if u.Host == "claude.ai" && strings.Contains(u.Path, "/artifact/") {
		return "[📄 page](" + target + ")"
	}
	return "[🔗 " + u.Host + "](" + target + ")"
}

// listBreaks puts a blank line before a list that follows a paragraph line, so every channel's
// parser reads it as a list and not as the paragraph's text.
func listBreaks(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inList := false
	for i, line := range lines {
		t := strings.TrimLeft(line, " \t")
		switch {
		case t == "":
			inList = false
		case listItem.MatchString(t):
			if !inList && i > 0 && strings.TrimSpace(lines[i-1]) != "" {
				out = append(out, "")
			}
			inList = true
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// alertFor builds the alert of sess's event; Root walks the open sessions up to the operator's child.
func (w *Worker) alertFor(sess store.Session, kind AlertKind, text, page string) Alert {
	root := w.root(sess)
	var remote string
	if p := w.panes[sess.ID]; p != nil {
		remote = p.remote
	}
	return Alert{
		Kind: kind, Project: w.Project, Title: sess.Title, Session: remote,
		Text: cut(text, alertRunes), Page: page, Root: root.ID,
	}
}

// root walks the open sessions up from sess to the operator's child; sess itself for a root.
func (w *Worker) root(sess store.Session) store.Session {
	for sess.Parent != store.Operator {
		up, ok := w.open[sess.Parent]
		if !ok {
			break
		}
		sess = up
	}
	return sess
}

// deliver posts a and logs it as action notify; a failure is logged once and false lets the caller
// retry next pass (R-WRK-083).
func (w *Worker) deliver(sess store.Session, a Alert) bool {
	if err := w.Notify(a); err != nil {
		w.Log.Error(sess.ID+" notify", err)
		return false
	}
	w.Log.Clear(sess.ID + " notify")
	w.Log.Action("notify", sess.ID, store.Excerpt(a.Text, len(a.Text)))
	return true
}

// once sends a Trouble or Your move alert unless the episode keyed by key was already told.
func (w *Worker) once(sess store.Session, p *pane, key string, kind AlertKind, text string) {
	if w.Notify == nil || p.alerted[key] {
		return
	}
	if w.deliver(sess, w.alertFor(sess, kind, text, "")) {
		if p.alerted == nil {
			p.alerted = map[string]bool{}
		}
		p.alerted[key] = true
	}
}

// alertItems alerts every unalerted open operator item and stamps the ones that went out
// (R-WRK-080, R-WRK-083); called once per pass. With no channel the items are stamped unsent, so
// enabling one later replays no backlog.
func (w *Worker) alertItems(now time.Time) {
	items, err := w.Store.UnalertedToOperator()
	if err != nil {
		w.Log.Error("notify", err)
		return
	}
	for _, it := range items {
		if w.Notify != nil && !w.alertItem(it) {
			continue
		}
		if err := w.Store.MarkAlerted(it.ID, now); err != nil {
			w.Log.Error(it.ID+" notify", err)
		}
	}
}

// alertItem sends the alert of one operator item; false when the post failed.
func (w *Worker) alertItem(it store.Item) bool {
	sess, ok := w.open[it.Sender]
	if !ok {
		var err error
		if sess, err = w.Store.Session(it.Sender); err != nil {
			sess = store.Session{ID: it.Sender, Title: "?", Parent: store.Operator}
		}
	}
	kind := AlertNeedsYou
	var text string
	switch {
	case it.Intent != store.IntentCheckpoint:
		text = firstLine(it.Body)
	case store.HasAsk(it.Body):
		text = strings.TrimSpace(strings.Join(store.SectionLines(it.Body, "Ask"), "\n")) // its lines kept
	default:
		kind, text = AlertCheckpoint, firstItem(store.SectionLines(it.Body, "Next"))
	}
	text = strings.ReplaceAll(strings.ToValidUTF8(text, "\uFFFD"), "\t", " ") // alertFor cuts it
	return w.deliver(sess, w.alertFor(sess, kind, text, store.PageOf(it.Body)))
}

// firstItem is the first item of a section's lines on one line: its first non-blank line and the
// continuation lines up to a blank line or a list item no more indented than it (R-WRK-080).
func firstItem(lines []string) string {
	var out []string
	indent := 0
	for _, line := range lines {
		t := strings.TrimLeft(line, " \t")
		switch {
		case len(out) == 0 && t == "":
		case len(out) == 0:
			out, indent = append(out, t), len(line)-len(t)
		case t == "" || listItem.MatchString(t) && len(line)-len(t) <= indent:
			return strings.Join(out, " ")
		default:
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

// listItem matches the marker that starts a Markdown list item: "- ", "* ", "+ " or "1. ".
var listItem = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s`)

// cut returns text whole when it fits in n runes; else its head up to the last sentence end
// (". ", "? ", "! ") or line end in the second half of the n runes, or failing that the last space,
// with "…" appended. It never cuts inside a link, URL, code span, bold or italic (spanRe): a
// word or span that leaves no other place is cut before the span, else at n (R-WRK-082).
// ponytail: a span that starts the text and is longer than n is still cut at n.
func cut(text string, n int) string {
	r := []rune(text)
	if len(r) <= n {
		return text
	}
	head := string(r[:n+1]) // the rune after the cap tells whether a sentence or word ends there
	spans := spanRe.FindAllStringIndex(text, -1)
	keep := lastFree(head, spans, func(i int) bool {
		end := head[i] == '\n' || head[i] == ' ' && strings.ContainsRune(".?!", rune(head[i-1]))
		return end && utf8.RuneCountInString(head[:i]) >= n/2
	})
	if keep < 0 {
		keep = lastFree(head, spans, func(i int) bool { return head[i] == ' ' })
	}
	if keep < 0 {
		keep = beforeSpan(len(string(r[:n])), spans)
	}
	return strings.TrimRight(head[:keep], " \n") + "…"
}

// beforeSpan moves the cut at byte keep to the start of the span it splits, unless that span starts
// the text.
func beforeSpan(keep int, spans [][]int) int {
	for _, s := range spans {
		if s[0] > 0 && s[0] < keep && keep < s[1] {
			return s[0]
		}
	}
	return keep
}

// lastFree is the last byte index i > 0 of head where ok holds and head[:i] splits no span; -1 when none.
func lastFree(head string, spans [][]int, ok func(i int) bool) int {
	for i := len(head) - 1; i > 0; i-- {
		if ok(i) && !slices.ContainsFunc(spans, func(s []int) bool { return s[0] < i && i < s[1] }) {
			return i
		}
	}
	return -1
}

// firstLine is the first non-blank line of body without leading # marks and spaces (R-WRK-080).
func firstLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if t := strings.TrimLeft(strings.TrimSpace(line), "# "); t != "" {
			return t
		}
	}
	return ""
}

// yourMove alerts once per idle episode a root that has been idle for Stall, waits on nothing that
// speaks for it and has nothing running below it (R-WRK-093). One cause, one alert: the root's open
// obligations and a descendant that owes its brief are covered by the stall reports.
func (w *Worker) yourMove(sess store.Session, p *pane, now time.Time) {
	if sess.Parent != store.Operator || w.Notify == nil || p.alerted["yourmove"] || p.background ||
		w.done(sess, p) || now.Sub(w.clock(sess, p)) < w.Stall {
		return
	}
	if w.moveCovered(sess, p, now) {
		return
	}
	w.once(sess, p, "yourmove", AlertYourMove,
		fmt.Sprintf("idle for %d minutes; nothing appears to be running", int(now.Sub(p.idleSince).Minutes())))
}

// done is a quick root's integration being its last act: a turn that began after it (a follow-up
// from the operator) makes the root a plain idle root again (R-WRK-098).
func (w *Worker) done(sess store.Session, p *pane) bool {
	return session.Done(sess) && !sess.IntegratedAt.Before(p.busyStart)
}

// moveCovered reports whether something else speaks for the idle root (R-WRK-093): a covering
// wait (any wait but an ask older than its latest turn: the root worked past it), an obligation
// whose stall report can fire (the root waits on nothing at all), or a descendant that is busy or
// whose owed-brief report can fire.
func (w *Worker) moveCovered(sess store.Session, p *pane, now time.Time) bool {
	waits, err := w.Store.Waits(sess.ID, now)
	if err != nil {
		w.Log.Error(sess.ID+" notify", err)
		return true
	}
	for _, wt := range waits {
		if wt.Kind != "ask" || w.askedThisTurn(sess, p, wt.Ref) {
			return true
		}
	}
	if len(waits) == 0 {
		owed, oerr := w.Store.Obligations(sess.ID)
		if oerr != nil {
			w.Log.Error(sess.ID+" notify", oerr)
			return true
		}
		if len(owed) > 0 {
			return true
		}
	}
	return w.treeBusy(sess.ID) || w.owesBelow(sess.ID, now)
}

// askedThisTurn reports whether the ask was sent in the root's latest turn: at or after its latest
// run of busy passes began, or before any was seen.
func (w *Worker) askedThisTurn(sess store.Session, p *pane, ref string) bool {
	ask, err := w.Store.Item(ref)
	if err != nil {
		w.Log.Error(sess.ID+" notify", err)
		return true
	}
	return !ask.CreatedAt.Before(p.busyStart)
}

// treeBusy reports whether a descendant of root is working, blocked, showing a background task or
// not seen yet this run.
func (w *Worker) treeBusy(root string) bool {
	for id, s := range w.open {
		if s.Parent != root {
			continue
		}
		p := w.panes[id]
		if p == nil || (p.status != "" && !idle(p.status)) || p.background || w.treeBusy(id) {
			return true
		}
	}
	return false
}

// owesBelow reports whether a descendant of root never handed off its brief and waits on nothing,
// so its own stall report can fire (R-WRK-050).
func (w *Worker) owesBelow(root string, now time.Time) bool {
	for id, s := range w.open {
		if s.Parent == root && (w.owesBrief(s) && !w.waits(s, now) || w.owesBelow(id, now)) {
			return true
		}
	}
	return false
}

// alertReports sends the Trouble alerts of the episodes the reports opened, a stall's at its second
// miss (R-WRK-094); a failed post is retried next pass without a second item for the manager.
func (w *Worker) alertReports(sess store.Session, p *pane, now time.Time) {
	if w.Notify != nil && !p.alerted["stall"] && w.secondMiss(sess, p, now) {
		w.once(sess, p, "stall", AlertTrouble, p.stallText)
	}
	if p.errored {
		w.once(sess, p, "error", AlertTrouble, fmt.Sprintf("shows %q on its screen", p.errName))
	}
	if !p.draft && !p.quiet(now, w.Quiet) { // Your move waits like a hint (R-WRK-093)
		w.yourMove(sess, p, now)
	}
}
