package testkit_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// doneAreas are the spec areas whose plans have landed (P1 to P6: every v1 area), plus UPD,
// which is red until its covering tests land.
var doneAreas = []string{
	"CFG", "SYNC", "PROF", "QA", "MSG", "HOOK", "MOD", "SESS", "DOC", "WRK", "NTF", "CTR", "PRB", "BLT", "STAT", "UPD",
}

// deferred names ids of done areas that a review entry, a local run or publication delivers.
var deferred = map[string]string{
	"R-BLT-056": "evals (local, D31)", "R-BLT-063": "evals (local, D31)", "R-BLT-073": "evals (local, D31)",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
}

func TestRequirementCoverage(t *testing.T) {
	// The gate for I4: every requirement of a done area has a covering test.
	c, err := testkit.ScanCoverage("../../docs/spec", "../..")
	if err != nil {
		t.Fatal(err)
	}
	if problems := c.Problems(doneAreas, deferred); len(problems) > 0 {
		t.Fatalf("requirement coverage:\n%s", strings.Join(problems, "\n"))
	}
}
