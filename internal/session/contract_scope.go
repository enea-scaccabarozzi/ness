package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// Lock tokens beyond a blob sha (R-CTR-031, R-CTR-033): a scenario-locked file holds
// "scn:<digest of its scenario lines>", a prefix-locked one "prefix:<tokens>:<sha256>".
const (
	// ScenarioTag starts the token of a scenario-locked file.
	ScenarioTag = "scn:"
	// PrefixTag starts the token of an append-only prefix-locked file.
	PrefixTag = "prefix:"
	// FormatTag starts the token of a file locked in a project-defined format (R-CTR-049):
	// "fmt:<format>:<digest of its unit lines>"; the units are stored as scenario lines.
	FormatTag = "fmt:"
)

// Diff is one frozen path that no longer holds; What says how when the lock can tell (a scenario
// edited or removed, a prefix broken), "" for a whole-file difference.
type Diff struct{ Path, What string }

// Lenient reports whether the lock token is a scenario, prefix or format lock, where additions
// are legal (R-CTR-031, R-CTR-033, R-CTR-050).
func Lenient(token string) bool {
	return strings.HasPrefix(token, ScenarioTag) || strings.HasPrefix(token, PrefixTag) ||
		strings.HasPrefix(token, FormatTag)
}

var gherkinKeyword = regexp.MustCompile(
	`^(Feature|Rule|Background|Scenario Outline|Scenario Template|Scenario|Example|Examples|Scenarios):\s*(.*)$`)

// FeatureScenarios parses Gherkin text into scenario name -> hash. A scenario's hash covers its
// keyword line, tags, steps, tables, Examples and doc strings (verbatim, indentation included),
// plus its context: the Feature's tags, the enclosing Rule's tags and every Background in scope.
// So a tag added above the Feature or a Rule, or a Background added or edited, shows as an edit of
// each locked scenario it reaches. Whitespace outside doc strings is collapsed and comments are
// dropped. A repeated name gets " #2", " #3"; an unnamed scenario is "<unnamed N>".
// ponytail: line-based, no Gherkin i18n keywords; swap in a real parser if a project needs them.
func FeatureScenarios(text string) map[string]string {
	g := &gherkin{out: map[string]string{}}
	for _, raw := range strings.Split(text, "\n") {
		g.feed(strings.TrimRight(raw, "\r"))
	}
	g.flush()
	return g.out
}

// gherkin is the parser state: the open block, the tags waiting for a keyword, the context lines
// of the Feature and of the enclosing Rule, and the open doc string's closing delimiter.
type gherkin struct {
	out           map[string]string
	pending, body []string
	feat, rule    []string
	name, doc     string
	background    bool
	open, inRule  bool
	unnamed       int
}

// flush closes the open block: a Background joins its context, a scenario goes into out.
func (g *gherkin) flush() {
	switch {
	case !g.open:
	case g.background && g.inRule:
		g.rule = append(g.rule, g.body...)
	case g.background:
		g.feat = append(g.feat, g.body...)
	default:
		key := g.name
		for n := 2; g.out[key] != ""; n++ {
			key = fmt.Sprintf("%s #%d", g.name, n)
		}
		g.out[key] = sum(strings.Join(g.feat, "\n") + "\x00" + strings.Join(g.rule, "\n") + "\x00" +
			strings.Join(g.body, "\n"))
	}
	g.open, g.body = false, nil
}

// feed consumes one raw line. Inside a doc string every line is scenario text, verbatim.
func (g *gherkin) feed(raw string) {
	t := strings.TrimSpace(raw)
	if g.doc != "" {
		g.add(raw)
		if t == g.doc {
			g.doc = ""
		}
		return
	}
	line := strings.Join(strings.Fields(raw), " ")
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}
	if d := docDelimiter(t); d != "" {
		g.doc = d
	}
	kw := gherkinKeyword.FindStringSubmatch(line)
	switch {
	case g.doc != "":
		g.add(line)
	case strings.HasPrefix(line, "@"):
		g.pending = append(g.pending, line)
	case kw == nil:
		g.add(line)
	default:
		g.keyword(kw[1], kw[2], line)
	}
}

// docDelimiter is the delimiter a line opens a doc string with, "" for any other line.
func docDelimiter(t string) string {
	for _, d := range []string{`"""`, "```"} {
		if strings.HasPrefix(t, d) {
			return d
		}
	}
	return ""
}

// add appends a line to the open block.
func (g *gherkin) add(line string) {
	if g.open {
		g.body = append(g.body, line)
	}
}

// keyword handles a keyword line: Examples extend the open block; Feature and Rule close it and
// set the context their tags belong to; anything else opens a block led by the tags waiting for it.
func (g *gherkin) keyword(kind, name, line string) {
	switch kind {
	case "Examples", "Scenarios":
		if g.open {
			g.body = append(append(g.body, g.pending...), line)
		}
	case "Feature":
		g.flush()
		g.feat, g.rule, g.inRule = g.pending, nil, false
	case "Rule":
		g.flush()
		g.rule, g.inRule = g.pending, true
	default:
		g.flush()
		if name == "" && kind != "Background" {
			g.unnamed++
			name = fmt.Sprintf("<unnamed %d>", g.unnamed)
		}
		g.name, g.open, g.background = name, true, kind == "Background"
		g.body = append(slices.Clone(g.pending), line)
	}
	g.pending = nil
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// scenarioDigest binds a file's scenario lines to its lock entry.
func scenarioDigest(m map[string]string) string {
	var b strings.Builder
	for _, n := range slices.Sorted(maps.Keys(m)) {
		fmt.Fprintf(&b, "%s\t%s\n", n, m[n])
	}
	return sum(b.String())
}

var (
	listMarker = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
	// Emphasis marks sit at a word's edge: leading, or trailing before punctuation; inside a
	// word (snake_case, a*b) they are text.
	leadMark  = regexp.MustCompile(`^[*_]+`)
	trailMark = regexp.MustCompile(`[*_]+([[:punct:]]*)$`)
	escape    = regexp.MustCompile(`\\([[:punct:]])`)
)

// mdTokens is the formatter-tolerant reading of Markdown: words only, so line wrapping, trailing
// spaces, list markers (-, *, +, 1., 1)), emphasis marks at a word's edge and backslash escapes
// do not count.
func mdTokens(text string) []string {
	toks := make([]string, 0, len(text)/6)
	for _, line := range strings.Split(text, "\n") {
		for _, w := range strings.Fields(listMarker.ReplaceAllString(strings.TrimSpace(line), "")) {
			w = trailMark.ReplaceAllString(leadMark.ReplaceAllString(escape.ReplaceAllString(w, "$1"), ""), "$1")
			if w != "" {
				toks = append(toks, w)
			}
		}
	}
	return toks
}

// prefixHolds reports whether text still starts with the words a "prefix:N:sum" token recorded.
func prefixHolds(token, text string) bool {
	n, want, _ := strings.Cut(strings.TrimPrefix(token, PrefixTag), ":")
	k, err := strconv.Atoi(n)
	toks := mdTokens(text)
	return err == nil && k <= len(toks) && sum(strings.Join(toks[:k], " ")) == want
}

func (l *Lock) init() {
	if l.Files == nil {
		l.Files = map[string]string{}
	}
	if l.Scenarios == nil {
		l.Scenarios = map[string]map[string]string{}
	}
}

// RecordScenarios locks path scenario by scenario (R-CTR-031).
func (l *Lock) RecordScenarios(root, path string) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // under the repo
	if err != nil {
		return err
	}
	l.recordScenarios(path, string(data))
	return nil
}

func (l *Lock) recordScenarios(path, text string) {
	l.init()
	m := FeatureScenarios(text)
	l.Files[path], l.Scenarios[path] = ScenarioTag+scenarioDigest(m), m
}

// RecordPrefix locks path's words as an append-only prefix (R-CTR-033).
func (l *Lock) RecordPrefix(root, path string) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) //nolint:gosec // under the repo
	if err != nil {
		return err
	}
	return l.recordPrefix(path, string(data))
}

func (l *Lock) recordPrefix(path, text string) error {
	l.init()
	toks := mdTokens(text)
	if len(toks) == 0 {
		return fmt.Errorf("%s has no text to lock as a prefix", path)
	}
	l.Files[path] = fmt.Sprintf("%s%d:%s", PrefixTag, len(toks), sum(strings.Join(toks, " ")))
	delete(l.Scenarios, path)
	return nil
}

// Set records a whole-file sha, or Amended, for path; any scenario lines it had go.
func (l *Lock) Set(path, sha string) {
	l.init()
	l.Files[path] = sha
	delete(l.Scenarios, path)
}

// lenientDiff reports how the file at path breaks its scenario, prefix or format lock, "" when it
// holds.
func lenientDiff(root string, f Frozen, c config.Contracts, x *Extractor) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path))) //nolint:gosec // under the repo
	if err != nil {
		return "file is missing"
	}
	if strings.HasPrefix(f.Sha, FormatTag) {
		return formatDiff(x, f, string(data), c)
	}
	if strings.HasPrefix(f.Sha, PrefixTag) {
		if prefixHolds(f.Sha, string(data)) {
			return ""
		}
		return "locked text is no longer the start of the file (append under an Amendment heading)"
	}
	now := FeatureScenarios(string(data))
	var bad []string
	for _, n := range slices.Sorted(maps.Keys(f.Scenarios)) {
		switch got, ok := now[n]; {
		case !ok:
			bad = append(bad, fmt.Sprintf("scenario %q removed", n))
		case got != f.Scenarios[n]:
			bad = append(bad, fmt.Sprintf("scenario %q edited", n))
		}
	}
	return strings.Join(bad, "; ")
}
