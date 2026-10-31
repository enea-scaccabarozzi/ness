// Package profiles embeds ness's core text and built-in profiles into the binary (R-PROF-002).
package profiles

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
)

//go:embed all:core all:builtin all:features
var embedded embed.FS

// Core returns the core profile's directory.
func Core() fs.FS { return sub("core") }

// Builtin returns the directory holding one subdirectory per built-in profile.
func Builtin() fs.FS { return sub("builtin") }

// Features returns the directory holding one subdirectory per feature with its SKILL.md
// (design §2: a feature brings its own skill text, rendered only while it is on).
func Features() fs.FS { return sub("features") }

func sub(dir string) fs.FS {
	s, err := fs.Sub(embedded, dir)
	if err != nil {
		panic(err) // the directory is embedded at build time; failure is a build defect
	}
	return s
}

// Names lists the built-in profiles, sorted (R-BLT-100).
func Names() ([]string, error) {
	entries, err := fs.ReadDir(Builtin(), ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Scan reports every file of fsys containing one of words, case-insensitively, as
// "path: word" (R-BLT-003, R-QA-048): the denylist check that keeps private names out of the
// built-ins and the public tree. Every run of whitespace, in the text and in the word, counts
// as one space, and so does a Markdown line break (a trailing backslash, a ">" continuation),
// so a word wrapped over a line still matches. A walk or read error is reported as a hit
// "walk: error" rather than hidden.
func Scan(fsys fs.FS, words []string) []string {
	var hits []string
	walk := func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return rerr
		}
		text := normalise(string(b))
		for _, w := range words {
			if strings.Contains(text, normalise(w)) {
				hits = append(hits, p+": "+w)
			}
		}
		return nil
	}
	if err := fs.WalkDir(fsys, ".", walk); err != nil {
		hits = append(hits, "walk: "+err.Error())
	}
	return hits
}

// lineBreak is a line break with its Markdown decoration: a hard-break backslash before it,
// blockquote markers and indent after it.
var lineBreak = regexp.MustCompile(`\\?\r?\n[\s>]*`)

// normalise lowercases s and collapses every line break and run of whitespace to one space.
func normalise(s string) string {
	return strings.Join(strings.Fields(lineBreak.ReplaceAllString(strings.ToLower(s), " ")), " ")
}
