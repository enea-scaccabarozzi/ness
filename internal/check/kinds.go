package check

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

// runKind runs a built-in kind. Anything it cannot decide fails (R-CHK-007).
func runKind(ctx Context, c profile.Check, deps Deps) (string, bool) {
	var err error
	switch c.Kind {
	case "section-matches":
		err = sectionMatches(ctx, c)
	case "branch-forbids":
		err = branchForbids(ctx, c, deps.Git)
	case "match-last":
		err = matchLast(ctx, c)
	case "checkpoint-since-landing":
		err = checkpointSinceLanding(ctx, deps.Facts)
	default:
		err = fmt.Errorf("unknown check kind %q", c.Kind)
	}
	if err != nil {
		return err.Error(), false
	}
	return "", true
}

// read returns a file below the worktree; a path that leaves it is an error.
func read(ctx Context, file string) (string, error) {
	root, err := os.OpenRoot(ctx.Worktree)
	if err != nil {
		return "", err
	}
	defer root.Close() //nolint:errcheck // read-only
	data, err := root.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", file, err)
	}
	return string(data), nil
}

// section returns the body of the first heading named name, and whether there was one.
func section(text, name string) (string, bool) {
	lines := strings.Split(text, "\n")
	hs := rule.HeadingLines([]byte(text))
	for i, h := range hs {
		if h.Text != name {
			continue
		}
		end := len(lines)
		for _, next := range hs[i+1:] {
			if next.Level <= h.Level {
				end = next.Line
				break
			}
		}
		return strings.Join(lines[h.Line+1:end], "\n"), true
	}
	return "", false
}

func sectionMatches(ctx Context, c profile.Check) error {
	file, heading := c.Args["file"], c.Args["heading"]
	text, err := read(ctx, file)
	if err != nil {
		return err
	}
	body, ok := section(text, heading)
	if !ok {
		return fmt.Errorf("%s has no heading %q", file, heading)
	}
	for _, req := range c.Require {
		re, err := regexp.Compile(req)
		if err != nil {
			return fmt.Errorf("require %q: %w", req, err)
		}
		if !re.MatchString(body) {
			return fmt.Errorf("%s: section %q does not match %s", file, heading, req)
		}
	}
	return nil
}

// addedLine is a line a diff adds to file.
type addedLine struct{ file, text string }

// addedLines parses a unified diff into the lines it adds (without the leading +).
func addedLines(diff string) (out []addedLine) {
	var file string
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file, inHunk = "", false
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case !inHunk && strings.HasPrefix(line, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
		case inHunk && strings.HasPrefix(line, "+"):
			out = append(out, addedLine{file, line[1:]})
		}
	}
	return out
}

func branchForbids(ctx Context, c profile.Check, git Git) error {
	path, err := regexp.Compile(strings.ReplaceAll(c.Args["path"], "{record}", regexp.QuoteMeta(ctx.Record)))
	if err != nil {
		return fmt.Errorf("path: %w", err)
	}
	pattern, err := regexp.Compile(c.Args["pattern"])
	if err != nil {
		return fmt.Errorf("pattern: %w", err)
	}
	diff, err := git.Out(ctx.Worktree, "-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff",
		"--no-textconv", "--src-prefix=a/", "--dst-prefix=b/", ctx.Into+"..."+ctx.Branch)
	if err != nil {
		return fmt.Errorf("git diff %s...%s: %w", ctx.Into, ctx.Branch, err)
	}
	for _, l := range addedLines(diff) {
		if path.MatchString(l.file) && pattern.MatchString(l.text) {
			return fmt.Errorf("%s adds %q", l.file, l.text)
		}
	}
	return nil
}

// lastGroup returns group 1 of the first (or last) match of pattern in the file.
func lastGroup(ctx Context, file, pattern string, last bool) (string, error) {
	text, err := read(ctx, file)
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("pattern %q: %w", pattern, err)
	}
	ms := re.FindAllStringSubmatch(text, -1)
	if len(ms) == 0 || len(ms[0]) < 2 {
		return "", fmt.Errorf("%s has no match for %s", file, pattern)
	}
	if last {
		return ms[len(ms)-1][1], nil
	}
	return ms[0][1], nil
}

func matchLast(ctx Context, c profile.Check) error {
	a := c.Args
	got, err := lastGroup(ctx, a["file"], a["pattern"], false)
	if err != nil {
		return err
	}
	want, err := lastGroup(ctx, a["against_file"], a["against_pattern"], true)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s says %s but %s ends at %s", a["file"], got, a["against_file"], want)
	}
	return nil
}

func checkpointSinceLanding(ctx Context, facts Facts) error {
	if ctx.Root == "" || ctx.Session != ctx.Root { // only a root can send a checkpoint
		return nil
	}
	landing, err := facts.LastLanding(ctx.Root)
	if err != nil {
		return err
	}
	if landing.IsZero() {
		return nil
	}
	cp, err := facts.LastCheckpoint(ctx.Root)
	if err != nil {
		return err
	}
	if cp.Before(landing) {
		return errors.New("no checkpoint to the operator since the last landing")
	}
	return nil
}
