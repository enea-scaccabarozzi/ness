package store

import (
	"fmt"
	"strings"
	"time"
)

var oneLine = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// Excerpt returns at most n runes of body on one line: newlines, carriage returns and tabs
// become spaces and invalid UTF-8 becomes U+FFFD (R-MSG-031). Shared by inbox and, in P4, the worker's hints.
func Excerpt(body string, n int) string {
	s := strings.ToValidUTF8(oneLine.Replace(body), "\uFFFD") // invalid bytes print the same at any length
	if runes := []rune(s); len(runes) > n {
		return string(runes[:n])
	}
	return s
}

// FormatAge prints d as its largest whole unit of minutes, hours or days, floored (R-MSG-045).
func FormatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
