package testkit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	defRe    = regexp.MustCompile(`(?m)^- \*\*(R-[A-Z]+-\d{3})\*\*`)
	coverRe  = regexp.MustCompile(`(?m)(?://|#)\s*covers:\s*(.+)$`)
	idRe     = regexp.MustCompile(`R-[A-Z]+-\d{3}`)
	reviewRe = regexp.MustCompile(`(?m)^- (R-[A-Z]+-\d{3}) \| ([^|\n]+) \| (\d{4}-\d{2}-\d{2}) \| ([0-9a-f]{7,40})\s*$`)
)

// Coverage maps requirement ids defined in the spec to what covers them.
type Coverage struct {
	defined map[string]bool
	covered map[string]bool
	named   []string // every id a marker or review entry names
}

// ScanCoverage reads definitions from specDir/*.md, review entries from specDir/reviews.md, and
// covers markers from every *_test.go and *.txtar under root (R-QA-010, R-QA-011, R-QA-012).
func ScanCoverage(specDir, root string) (*Coverage, error) {
	c := &Coverage{defined: map[string]bool{}, covered: map[string]bool{}}
	specs, err := filepath.Glob(filepath.Join(specDir, "*.md"))
	if err != nil {
		return nil, err
	}
	for _, p := range specs {
		b, err := os.ReadFile(p) //nolint:gosec // paths come from a glob of the spec dir
		if err != nil {
			return nil, err
		}
		for _, m := range defRe.FindAllStringSubmatch(string(b), -1) {
			c.defined[m[1]] = true
		}
		for _, m := range reviewRe.FindAllStringSubmatch(string(b), -1) {
			c.mark(m[1])
		}
	}
	if err := c.walkMarkers(root); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Coverage) walkMarkers(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(p, "_test.go") && !strings.HasSuffix(p, ".txtar") {
			return nil
		}
		b, rdErr := os.ReadFile(p) //nolint:gosec // walking the repository
		if rdErr != nil {
			return rdErr
		}
		for _, m := range coverRe.FindAllStringSubmatch(string(b), -1) {
			for _, id := range idRe.FindAllString(m[1], -1) {
				c.mark(id)
			}
		}
		return nil
	})
}

func (c *Coverage) mark(id string) {
	c.covered[id] = true
	c.named = append(c.named, id)
}

// Problems lists defined ids of the given areas with no cover (unless deferred) and ids named
// anywhere but not defined (R-QA-013), sorted.
func (c *Coverage) Problems(areas []string, deferred map[string]string) []string {
	var out []string
	for id := range c.defined {
		area := strings.Split(id, "-")[1]
		if slices.Contains(areas, area) && !c.covered[id] && deferred[id] == "" {
			out = append(out, id+" has no covering test or review entry")
		}
	}
	for _, id := range c.named {
		if !c.defined[id] {
			out = append(out, id+" is named by a covers marker or review entry but not defined in docs/spec")
		}
	}
	for id, plan := range deferred {
		if c.covered[id] {
			out = append(out, fmt.Sprintf("%s is deferred to %s but is covered; remove it from deferred", id, plan))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
