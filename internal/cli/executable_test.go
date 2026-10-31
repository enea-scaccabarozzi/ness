package cli

import "testing"

func TestTrimDeletedFindsAReplacedBinary(t *testing.T) {
	// covers: R-WRK-009
	cases := map[string]string{
		"/usr/local/bin/ness (deleted)": "/usr/local/bin/ness",
		"/usr/local/bin/ness":           "/usr/local/bin/ness",
		"/opt/a (deleted) b":            "/opt/a (deleted) b",
	}
	for in, want := range cases {
		if got := trimDeleted(in); got != want {
			t.Errorf("trimDeleted(%q) = %q, want %q", in, got, want)
		}
	}
}
