package config_test

import (
	"strings"
	"testing"
)

func TestContractFormatsValidateAndKeepConfigOrder(t *testing.T) {
	// A [contracts.formats.NAME] needs a lowercase name, globs without ** and a units command;
	// the formats keep the order the file wrote them in.
	// covers: R-CTR-047
	c, err := load(t, minimal+`
[contracts.formats.zeta]
match = ["tests/test_*.py"]
units = "tools/units.py"
[contracts.formats.alpha]
match = ["*.feature"]
units = "python3 -c 'print(1)'"
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.Contracts.FormatOrder, ","); got != "zeta,alpha" {
		t.Errorf("FormatOrder = %q; want zeta,alpha", got)
	}
	if name, _, ok := c.Contracts.FormatOf("x.feature"); !ok || name != "alpha" {
		t.Errorf("FormatOf(x.feature) = %q, %v", name, ok)
	}
	for body, want := range map[string]string{
		"[contracts.formats.Bad]\nmatch = [\"a\"]\nunits = \"x\"\n":              "contracts.formats.Bad",
		"[contracts.formats.a]\nunits = \"x\"\n":                                 "contracts.formats.a.match",
		"[contracts.formats.a]\nmatch = [\"a\"]\n":                               "contracts.formats.a.units",
		"[contracts.formats.a]\nmatch = [\"**/a\"]\nunits = \"x\"\n":             "**",
		"[contracts.formats.a]\nmatch = [\"[\"]\nunits = \"x\"\n":                "not a valid glob",
		"[contracts.formats.a]\nmatch = [\"a\"]\nunits = \"x\"\nextra = \"y\"\n": "unknown key",
	} {
		mustFail(t, minimal+body, want)
	}
}

func TestContractFormatFromProfileDefaultsMatches(t *testing.T) {
	// A format that comes from a profile default, not the file, is in the match order, after the
	// file's own.
	// covers: R-CTR-047
	c, err := load(t, minimal+"[contracts.formats.own]\nmatch = [\"*.own\"]\nunits = \"x\"\n", map[string]any{
		"contracts.formats.p": map[string]any{"match": []any{"*.p"}, "units": "y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.Contracts.FormatOrder, ","); got != "own,p" {
		t.Errorf("FormatOrder = %q; want own,p", got)
	}
	if name, _, ok := c.Contracts.FormatOf("a.p"); !ok || name != "p" {
		t.Errorf("FormatOf(a.p) = %q, %v", name, ok)
	}
}
