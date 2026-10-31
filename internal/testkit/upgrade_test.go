package testkit_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestPatchRule(t *testing.T) {
	// covers: R-UPD-001
	for _, c := range []struct {
		manifest, fixture string
		fs, s             int
		bad               bool
	}{
		{"0.1.1", "0.1.0", 12, 13, true},
		{"0.1.1", "0.1.0", 12, 12, false},
		{"0.2.0", "0.1.0", 12, 13, false},
		{"0.1.0", "0.1.0", 12, 13, false}, // the fixture's own release: no patch over it
	} {
		err := testkit.PatchRule(c.manifest, c.fixture, c.fs, c.s)
		if (err != nil) != c.bad {
			t.Errorf("%+v: %v", c, err)
		}
		if c.bad && !strings.Contains(err.Error(), "adds a store migration") {
			t.Errorf("%+v: %v", c, err)
		}
	}
}
