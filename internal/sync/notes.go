package sync

import (
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
// it). A "## " heading that is not a version is skipped with its text; a version without a
// "### Running sessions" subsection is an error (R-SYNC-089).
func Notes(changelog string) ([]Note, error) {
	var out []Note
	var cur *Note
	flush := func() error {
		if cur == nil {
			return nil
		}
		if !strings.Contains(cur.Body, "### Running sessions") {
			return fmt.Errorf("CHANGELOG %s has no \"### Running sessions\" subsection", cur.Version)
		}
		out = append(out, Note{Version: cur.Version, Body: strings.TrimSpace(cur.Body) + "\n"})
		cur = nil
		return nil
	}
	for _, l := range strings.Split(changelog, "\n") {
		if strings.HasPrefix(l, "## ") {
			if err := flush(); err != nil {
				return nil, err
			}
			if m := versionRe.FindStringSubmatch(l); m != nil {
				cur = &Note{Version: m[1]}
			}
			continue
		}
		if cur != nil {
			cur.Body += l + "\n"
		}
	}
	return out, flush()
}

// Between returns the notes of the versions after from and up to to, sorted oldest first
// (R-SYNC-032); none when they are equal (R-SYNC-033) or to is older. A prerelease sorts below
// its release; a version that is not X.Y.Z[-pre] is an error.
func Between(notes []Note, from, to string) ([]Note, error) {
	f, err := parseVersion(from)
	if err != nil {
		return nil, err
	}
	t, err := parseVersion(to)
	if err != nil {
		return nil, err
	}
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
		if f.less(v) && !t.less(v) {
			in = append(in, item{n, v})
		}
	}
	slices.SortFunc(in, func(a, b item) int {
		switch {
		case a.v.less(b.v):
			return -1
		case b.v.less(a.v):
			return 1
		}
		return 0
	})
	out := make([]Note, len(in))
	for i, x := range in {
		out[i] = x.n
	}
	return out, nil
}

type version struct {
	num [3]int
	pre string
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+.*)?$`)

func parseVersion(s string) (version, error) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return version{}, fmt.Errorf("version %q is not X.Y.Z", s)
	}
	v := version{pre: m[4]}
	for i := range 3 {
		v.num[i], _ = strconv.Atoi(m[i+1]) //nolint:errcheck // the regexp matched digits
	}
	return v, nil
}

// less orders by number, then a prerelease below its release (ponytail: prereleases compare as
// plain strings, not by semver identifier rules).
func (a version) less(b version) bool {
	if a.num != b.num {
		for i := range 3 {
			if a.num[i] != b.num[i] {
				return a.num[i] < b.num[i]
			}
		}
	}
	switch {
	case a.pre == b.pre:
		return false
	case a.pre == "":
		return false
	case b.pre == "":
		return true
	}
	return a.pre < b.pre
}
