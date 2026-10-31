package sync

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Note is one version's release notes, its "### Running sessions" subsection included.
type Note struct {
	Version, Body string
}

var versionRe = regexp.MustCompile(`^## \[?v?(\d+\.\d+\.\d+)`)

// Notes splits a CHANGELOG into versions in file order (newest first as release-please writes
// it). A "## " heading that is not a version is skipped with its text. A version without a
// "### Running sessions" subsection is an error (R-SYNC-089); the notes still hold every version,
// so a reader that needs only some of them (the worker's update item) can go on.
func Notes(changelog string) ([]Note, error) {
	var out []Note
	var errs []error
	var cur *Note
	flush := func() {
		if cur == nil {
			return
		}
		if !strings.Contains(cur.Body, "### Running sessions") {
			errs = append(errs, fmt.Errorf("CHANGELOG %s has no \"### Running sessions\" subsection", cur.Version))
		}
		out = append(out, Note{Version: cur.Version, Body: strings.TrimSpace(cur.Body) + "\n"})
		cur = nil
	}
	for _, l := range strings.Split(changelog, "\n") {
		if strings.HasPrefix(l, "## ") {
			flush()
			if m := versionRe.FindStringSubmatch(l); m != nil {
				cur = &Note{Version: m[1]}
			}
			continue
		}
		if cur != nil {
			cur.Body += l + "\n"
		}
	}
	flush()
	return out, errors.Join(errs...)
}

// Between returns the notes of the versions after from and up to to, sorted oldest first
// (R-SYNC-032); none when they are equal (R-SYNC-033) or to is older. A prerelease bound counts
// as its release: binary X.Y.Z-rc.N prints the X.Y.Z notes, and a lock of X.Y.Z-rc.N does not
// print them again. A version that is not X.Y.Z[-pre] is an error.
func Between(notes []Note, from, to string) ([]Note, error) {
	f, err := parseVersion(from)
	if err != nil {
		return nil, err
	}
	t, err := parseVersion(to)
	if err != nil {
		return nil, err
	}
	f.pre, t.pre = nil, nil
	type item struct {
		n Note
		v version
	}
	var in []item
	for _, n := range notes {
		v, err := parseVersion(n.Version)
		if err != nil {
			return nil, err
		}
		if f.cmp(v) < 0 && t.cmp(v) >= 0 {
			in = append(in, item{n, v})
		}
	}
	slices.SortFunc(in, func(a, b item) int { return a.v.cmp(b.v) })
	out := make([]Note, len(in))
	for i, x := range in {
		out[i] = x.n
	}
	return out, nil
}

// RunningSessions returns the body of n's "### Running sessions" subsection, trimmed: the text
// up to the next "### " heading or the end.
func RunningSessions(n Note) string {
	_, after, ok := strings.Cut(n.Body, "### Running sessions\n")
	if !ok {
		return ""
	}
	if i := strings.Index(after, "\n### "); i >= 0 {
		after = after[:i]
	}
	return strings.TrimSpace(after)
}

// Compare orders two versions by SemVer 2.0 precedence (-1, 0 or 1): a prerelease sorts below
// its release, digit-only prerelease identifiers compare as numbers, build metadata is ignored.
// A leading "v" is allowed; a version that is not X.Y.Z[-pre][+build] is an error.
func Compare(a, b string) (int, error) {
	va, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	return va.cmp(vb), nil
}

type version struct {
	num [3]int
	pre []string
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+.*)?$`)

func parseVersion(s string) (version, error) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return version{}, fmt.Errorf("version %q is not X.Y.Z", s)
	}
	var v version
	if m[4] != "" {
		v.pre = strings.Split(m[4], ".")
	}
	for i := range 3 {
		v.num[i], _ = strconv.Atoi(m[i+1]) //nolint:errcheck // the regexp matched digits
	}
	return v, nil
}

func (a version) cmp(b version) int {
	if c := slices.Compare(a.num[:], b.num[:]); c != 0 {
		return c
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := range min(len(a.pre), len(b.pre)) {
		if c := cmpIdent(a.pre[i], b.pre[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.pre), len(b.pre))
}

// cmpIdent orders two prerelease identifiers: numbers numerically and below any alphanumeric.
func cmpIdent(a, b string) int {
	an, bn := digits(a), digits(b)
	switch {
	case an && bn:
		return cmp.Or(cmp.Compare(len(strings.TrimLeft(a, "0")), len(strings.TrimLeft(b, "0"))),
			strings.Compare(strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")))
	case an:
		return -1
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

func digits(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }
