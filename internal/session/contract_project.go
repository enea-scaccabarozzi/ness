package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// LockKind is the lock token kind a path freezes as: "prefix:" for a .md file matching one of the
// prefix globs (R-CTR-033), "scn:" for any other .feature file (R-CTR-031), "" for whole-file.
// Session freeze and project-wide [contracts] frozen share it (R-CTR-040).
func LockKind(rel string, prefix []string) string {
	switch {
	case strings.HasSuffix(rel, ".md") && MatchAny(prefix, rel):
		return PrefixTag
	case strings.HasSuffix(rel, ".feature"):
		return ScenarioTag
	}
	return ""
}

// MatchAny reports whether rel matches one of the path.Match globs; a bad glob matches nothing.
func MatchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if ok, err := path.Match(g, rel); err == nil && ok {
			return true
		}
	}
	return false
}

// isGlob reports whether a [contracts] frozen entry holds path.Match metacharacters.
func isGlob(p string) bool { return strings.ContainsAny(p, `*?[\`) }

// ProjectFrozen turns the [contracts] frozen entries into frozen paths (R-CTR-040..042). A glob
// stands for every file at base that it matches (a file new on the branch is ProjectAdded's); a
// literal entry stands for itself. A .feature path is scenario-locked and a prefix-glob .md path
// prefix-locked against its text at base, like a session lock; the rest are whole-file, compared
// with the blob at base. A literal absent at base is left whole-file, where CheckContracts fails
// on it. A path in a project-defined format (c.Formats, matched before the builtins) is locked unit
// by unit from its text at base, run by x (R-CTR-053); a units command that fails is an error.
func ProjectFrozen(g Git, root, base string, c config.Contracts, x *Extractor) (map[string]Frozen, error) {
	names, err := ProjectPaths(g, root, base, c.Frozen)
	if err != nil {
		return nil, err
	}
	out := map[string]Frozen{}
	for _, n := range names {
		name, f, ok := c.FormatOf(n)
		if !ok {
			out[n] = projectEntry(g, root, base, n, c.Prefix)
			continue
		}
		fr, ferr := formatEntry(g, root, base, n, name, f, x)
		if ferr != nil {
			return nil, ferr
		}
		out[n] = fr
	}
	return out, nil
}

// ProjectPaths expands the entries: a literal stands for itself, a glob for the files of base it
// matches.
func ProjectPaths(g Git, root, base string, patterns []string) ([]string, error) {
	var names, tree []string
	for _, p := range patterns {
		if !isGlob(p) {
			names = append(names, p)
			continue
		}
		if tree == nil {
			ls, err := g.Out(root, "ls-tree", "-r", "-z", "--name-only", base)
			if err != nil {
				return nil, fmt.Errorf("ls-tree %s: %w", base, err)
			}
			tree = strings.Split(ls, "\x00")
		}
		for _, t := range tree {
			if ok, err := path.Match(p, t); err == nil && ok && t != "" {
				names = append(names, t)
			}
		}
	}
	return names, nil
}

// projectEntry is the frozen state of one project-wide path, read from its text at base. A path
// that is absent at base (CheckContracts reports it) or has no words to lock as a prefix stays
// whole-file.
func projectEntry(g Git, root, base, p string, prefix []string) Frozen {
	kind := LockKind(p, prefix)
	if text, err := g.Out(root, "cat-file", "blob", base+":"+p); kind != "" && err == nil {
		var l Lock
		if kind == ScenarioTag {
			l.recordScenarios(p, text)
			return Frozen{Path: p, Sha: l.Files[p], Scenarios: l.Scenarios[p]}
		}
		if l.recordPrefix(p, text) == nil {
			return Frozen{Path: p, Sha: l.Files[p]}
		}
	}
	return Frozen{Path: p}
}

// ProjectAdded maps each file of the worktree at root (tracked or untracked, not ignored) that a
// glob of patterns matches and base lacks to the first such glob (R-CTR-055).
func ProjectAdded(g Git, root, base string, patterns []string) (map[string]string, error) {
	names, err := ProjectPaths(g, root, base, patterns)
	if err != nil {
		return nil, err
	}
	ls, err := g.Out(root, "ls-files", "-z", "-co", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("ls-files: %w", err)
	}
	out := map[string]string{}
	for _, f := range strings.Split(ls, "\x00") {
		if f == "" || slices.Contains(names, f) {
			continue
		}
		glob := func(p string) bool { return isGlob(p) && MatchAny([]string{p}, f) }
		if i := slices.IndexFunc(patterns, glob); i >= 0 {
			out[f] = patterns[i]
		}
	}
	return out, nil
}

// AtBaseTip reports whether the worktree's p is approved at a tip of base (R-CTR-057): its content
// is byte-identical to p at refs/heads/<base> when the merge-base mb is its ancestor, or at
// refs/remotes/origin/<base> when the local base is its ancestor (origin level or ahead, never
// behind or diverged); or p is gone from the worktree and absent at such a tip. Full ref names, so a
// tag or other ref named like base shadows neither. The refs are read as they are, never fetched.
// It covers a branch whose link to base was lost (a squash fold) or a stale local base. Any error
// is false: the caller goes on to its own verdict.
func AtBaseTip(g Git, root, base, mb, p string) bool {
	_, serr := os.Lstat(filepath.Join(root, filepath.FromSlash(p)))
	gone := errors.Is(serr, fs.ErrNotExist)
	got, herr := g.BlobSha(root, p)
	local, origin := "refs/heads/"+base, "refs/remotes/origin/"+base
	for _, tip := range [][2]string{{mb, local}, {local, origin}} { // {floor, tip}
		if _, err := g.Out(root, "merge-base", "--is-ancestor", tip[0], tip[1]); err != nil {
			continue
		}
		want, berr := g.BaseBlob(root, tip[1], p)
		if (gone && berr != nil) || (!gone && herr == nil && berr == nil && got == want) {
			return true
		}
	}
	return false
}
