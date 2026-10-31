// Package squash is the integrator that folds a branch as one commit on the target
// (R-SESS-095), answering integrated? by a trailer naming the source sha.
package squash

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Trailer names the folded source commit in the squash commit's message.
const Trailer = "Ness-Integrates"

// Integrator implements session.Integrator. Subject is the squash commit's subject (R-SESS-140);
// empty falls back to "integrate SOURCE into TARGET".
type Integrator struct{ Subject string }

var treeLine = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// Fold squashes source onto the throwaway's HEAD as one commit and checks it out there. A source
// squashed before folds from the source sha its newest trailer on the target names, so landed
// changes are not applied twice (R-SESS-142).
func (it Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	src, err := g.Rev(dir, source)
	if err != nil {
		return "", err
	}
	tree, last, err := mergeTree(g, dir, src)
	if err != nil {
		return "", err
	}
	if head, herr := g.Out(dir, "rev-parse", "HEAD^{tree}"); herr != nil || head == tree {
		if herr != nil {
			return "", herr
		}
		return "", fmt.Errorf("%w: %s changes nothing in %s", session.ErrNothingToIntegrate, source, target)
	}
	msg, err := it.message(g, dir, source, target, src, last)
	if err != nil {
		return "", err
	}
	commit, err := g.Out(dir, "commit-tree", tree, "-p", "HEAD", "-m", msg)
	if err != nil {
		return "", err
	}
	_, err = g.Out(dir, "reset", "-q", "--hard", commit) // the gate runs on the folded tree
	return commit, err
}

// mergeTree merges src into HEAD without touching the worktree and returns the tree and the
// source sha of src's last squash on HEAD ("" when none). A conflict is ErrConflict naming the paths.
func mergeTree(g session.Git, dir, src string) (tree, last string, err error) {
	squashed, last, err := lastSquash(g, dir, src)
	if err != nil {
		return "", "", err
	}
	args := []string{"merge-tree", "--write-tree", "--name-only", "--no-messages"}
	if last != "" {
		// Unless the source already merged that squash, git's own merge base predates it.
		merged, aerr := g.IsAncestor(dir, squashed, src)
		if aerr != nil {
			return "", "", aerr
		}
		if !merged {
			args = append(args, "--merge-base="+last)
		}
	}
	tree, err = g.Out(dir, append(args, "HEAD", src)...)
	if err != nil {
		if lines := strings.Split(tree, "\n"); len(lines) > 1 && treeLine.MatchString(lines[0]) {
			return "", "", fmt.Errorf("%w: %s", session.ErrConflict, strings.Join(lines[1:], ", "))
		}
		return "", "", err
	}
	return tree, last, nil
}

// lastSquash finds the newest squash commit on HEAD whose trailer names an ancestor of src, and
// that sha ("" when src was never squashed here).
func lastSquash(g session.Git, dir, src string) (commit, sha string, err error) {
	out, err := g.Out(dir, "log", "--fixed-strings", "--grep", Trailer+": ",
		"--format=%H %(trailers:key="+Trailer+",valueonly,separator=%x20)", "HEAD")
	if err != nil || out == "" {
		return "", "", err
	}
	revs, err := g.Out(dir, "rev-list", src)
	if err != nil {
		return "", "", err
	}
	ancestors := map[string]bool{}
	for r := range strings.FieldsSeq(revs) {
		ancestors[r] = true
	}
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.Fields(line)
		for _, s := range f[min(1, len(f)):] {
			if ancestors[s] {
				return f[0], s, nil
			}
		}
	}
	return "", "", nil
}

// message is the subject, each folded commit's subject as a release-please nested commit, oldest
// first and merges left out, then the trailer (R-SESS-141).
func (it Integrator) message(g session.Git, dir, source, target, src, last string) (string, error) {
	args := []string{"log", "--reverse", "--no-merges", "--format=%s", src, "^HEAD"}
	if last != "" {
		args = append(args, "^"+last)
	}
	out, err := g.Out(dir, args...)
	if err != nil {
		return "", err
	}
	subject := it.Subject
	if subject == "" {
		subject = "integrate " + source + " into " + target
	}
	parts := []string{subject}
	if out != "" {
		const begin, end = "BEGIN_NESTED_COMMIT\n", "\nEND_NESTED_COMMIT"
		nested := begin + strings.ReplaceAll(out, "\n", end+"\n"+begin) + end
		parts = append(parts, nested)
	}
	return strings.Join(append(parts, Trailer+": "+src), "\n\n"), nil
}

// Integrated looks for the trailer on the target.
func (Integrator) Integrated(g session.Git, repo, source, target string) (bool, error) {
	src, err := g.Rev(repo, source)
	if err != nil {
		return false, err
	}
	out, err := g.Out(repo, "log", "--format=%H", "--fixed-strings", "--grep", Trailer+": "+src, "-n", "1", target)
	return out != "", err
}
