package session_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

func TestStripANSIRemovesEveryEscapeKind(t *testing.T) {
	// CSI (colors, cursor moves), OSC (titles, hyperlinks), charset designations and two-byte
	// escapes go; text stays, including a bare ">" and box drawing.
	// covers: R-WRK-015
	for in, want := range map[string]string{
		"\x1b[2mTry \"fix\"\x1b[22m":                `Try "fix"`,
		"\x1b[38;5;240m│\x1b[0m > hello":             "│ > hello",
		"\x1b]0;claude\x07x\x1b]8;;http://a\x1b\\y": "xy",
		"\x1b]8;;http://a\x1b\\LINK\x1b]8;;\x1b\\ after": "LINK after",
		"\x1b[?25l\x1b[1;1H\x1b[Ktext\x1b(B":        "text",
		"plain > text":                              "plain > text",
	} {
		if got := session.StripANSI(in); got != want {
			t.Errorf("StripANSI(%q) = %q, want %q", in, got, want)
		}
	}
}
