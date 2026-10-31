package store_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestFormatAge(t *testing.T) {
	// covers: R-MSG-045
	cases := map[time.Duration]string{
		0:                               "0m",
		59 * time.Second:                "0m",
		61 * time.Second:                "1m",
		3*time.Hour + 10*time.Minute:    "3h",
		50 * time.Hour:                  "2d",
		-time.Minute:                    "0m",
		60 * time.Second:                "1m",
		59*time.Minute + 59*time.Second: "59m",
		time.Hour:                       "1h",
		23*time.Hour + 59*time.Minute:   "23h",
		24 * time.Hour:                  "1d",
	}
	for d, want := range cases {
		if got := store.FormatAge(d); got != want {
			t.Errorf("FormatAge(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestExcerpt(t *testing.T) {
	// covers: R-MSG-031
	cases := []struct{ in, want string }{
		{"short", "short"},
		{"a\nb\tc\r\nd", "a b c d"},
		{strings.Repeat("é", 100), strings.Repeat("é", 80)},
		{"", ""},
		{strings.Repeat("a", 80), strings.Repeat("a", 80)},
		{strings.Repeat("a", 81), strings.Repeat("a", 80)},
		{strings.Repeat("a", 79) + "é" + "é", strings.Repeat("a", 79) + "é"},
		{strings.Repeat("a", 79) + "日本", strings.Repeat("a", 79) + "日"},
		{"a\xffb", "a\uFFFDb"},
	}
	for _, c := range cases {
		if got := store.Excerpt(c.in, 80); got != c.want {
			t.Errorf("Excerpt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func FuzzExcerpt(f *testing.F) {
	// Review Focus 2: arbitrary bytes never break the one-line, 80-rune promise.
	// covers: R-MSG-031
	f.Add("hello\nworld\tagain")
	f.Add(strings.Repeat("é", 100))
	f.Add("\xff\xfe invalid utf-8 \xc0")
	f.Add("")
	f.Fuzz(func(t *testing.T, body string) {
		got := store.Excerpt(body, 80)
		if strings.ContainsAny(got, "\n\r\t") {
			t.Fatalf("excerpt %q spans lines", got)
		}
		if utf8.RuneCountInString(got) > 80 {
			t.Fatalf("excerpt has %d runes", utf8.RuneCountInString(got))
		}
		if !utf8.ValidString(got) {
			t.Fatalf("excerpt %q is not valid UTF-8", got)
		}
	})
}
