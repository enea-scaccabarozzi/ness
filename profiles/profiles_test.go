package profiles_test

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/profiles"
)

// leak is one generic leak pattern of the public scan (R-BLT-003). Product names are not
// listed here: they live in the gitignored .private/denylist.txt. ok, when set, excuses a match.
type leak struct {
	name string
	re   *regexp.Regexp
	ok   func(match string) bool
}

var (
	exampleURL    = regexp.MustCompile(`(?i)^https?://([a-z0-9-]+\.)*example\.(com|org)([/:]|$)`)
	exampleMail   = regexp.MustCompile(`(?i)@([a-z0-9-]+\.)*example\.(com|org)$`)
	privateAddr   = regexp.MustCompile(`^(10\.\d+|192\.168|172\.(1[6-9]|2\d|3[01]))\.\d+\.\d+$`)
	fixedDenylist = []leak{
		{"url", regexp.MustCompile(`https?://[^\s)>"'` + "`" + `;,]+`), exampleURL.MatchString},
		{"email", regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`), exampleMail.MatchString},
		{"token", regexp.MustCompile(`\bghp_|\bxox[abp]-|\bsk-[A-Za-z0-9]{8}|\bAKIA[0-9A-Z]{8}`), nil},
		{"long run", regexp.MustCompile(`[0-9a-fA-F]{40,}|[A-Za-z0-9+]{40,}={0,2}`), nil},
		// Only a run of exactly four numbers can be an address; the excuse is "not a private one".
		{"rfc1918", regexp.MustCompile(`\d+(\.\d+){3,}`), func(m string) bool { return !privateAddr.MatchString(m) }},
		// lan and internal are never code; local and home are (this.local, user.home), so they
		// count only inside a URL or an address.
		{"internal tld", regexp.MustCompile(`(?i)(//|@)[\w.-]+\.(lan|local|internal|home)\b|\w\.(lan|internal)\b`), nil},
	}
)

// scanFixed returns "file: class: match" for every generic leak in fsys; a walk or read error
// fails the test, so an unreadable file cannot pass the scan.
func scanFixed(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	var hits []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return rerr
		}
		for _, l := range fixedDenylist {
			for _, m := range l.re.FindAllString(string(b), -1) {
				if l.ok == nil || !l.ok(m) {
					hits = append(hits, p+": "+l.name+": "+m)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanFixed: %v", err)
	}
	return hits
}

func TestFiveBuiltins(t *testing.T) {
	// covers: R-BLT-100
	names, err := profiles.Names()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"behavior-specs", "example-release", "git-conventional", "go-strict", "program"}
	if !slices.Equal(names, want) {
		t.Fatalf("built-ins = %v, want %v", names, want)
	}
}

func TestSkillsCarryThePrefix(t *testing.T) {
	// Every skill directory of core and the built-ins is ness-<name>, and its frontmatter name
	// equals the directory (R-BLT-006). Feature skills are named by the renderer (ness-<feature>).
	// covers: R-BLT-006
	for _, fsys := range []fs.FS{profiles.Core(), profiles.Builtin()} {
		err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
				return err
			}
			dir := path.Base(path.Dir(p))
			b, rerr := fs.ReadFile(fsys, p)
			if rerr != nil {
				return rerr
			}
			if !strings.HasPrefix(dir, "ness-") {
				t.Errorf("%s: skill directory %q lacks the ness- prefix", p, dir)
			}
			if !strings.Contains(string(b), "\nname: "+dir+"\n") {
				t.Errorf("%s: frontmatter name is not %q", p, dir)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDenylistScan(t *testing.T) {
	// A planted entry is found; the real texts contain none of the generic leak patterns; the
	// private list is scanned when present and skipped when absent (R-BLT-003, R-BLT-092).
	// covers: R-BLT-003, R-BLT-092, R-QA-048
	planted := fstest.MapFS{
		"x/profile.toml": {Data: []byte("name = \"x\"\n")},
		"x/text.md":      {Data: []byte("Deploy to Acme-Prod at 10:00\n")},
	}
	if hits := profiles.Scan(planted, []string{"acme-prod", "nothing"}); !slices.Equal(hits, []string{"x/text.md: acme-prod"}) {
		t.Fatalf("Scan(planted) = %v", hits)
	}
	wrapped := fstest.MapFS{
		"quote.md": {Data: []byte("> runs on Big\n> Widget Co hosts\n")},
		"hard.md":  {Data: []byte("the Small \\\nGadget\tline\n")},
		"split.md": {Data: []byte("the Small Gad get\n")},
	}
	got := profiles.Scan(wrapped, []string{"big widget  co", "small\ngadget line"})
	if want := []string{"hard.md: small\ngadget line", "quote.md: big widget  co"}; !slices.Equal(got, want) {
		t.Fatalf("Scan(wrapped) = %q, want %q", got, want)
	}
	leaky := fstest.MapFS{"a.md": {Data: []byte(
		"ok https://example.com/a http://www.example.org:80/ https://docs.example.com mail a@example.org\n" +
			"bad https://corp.acme.test/x http://example.com.evil.net/x b@acme.test a@example.attacker.com\n" +
			"bad 10.1.2.3 NAS.LAN http://db.internal key ghp_abc\n" +
			"ok this.local user.home 1.10.2.3.4\n")}}
	want := []string{
		"a.md: url: https://corp.acme.test/x",
		"a.md: url: http://example.com.evil.net/x",
		"a.md: url: http://db.internal",
		"a.md: email: b@acme.test",
		"a.md: email: a@example.attacker.com",
		"a.md: token: ghp_",
		"a.md: rfc1918: 10.1.2.3",
		"a.md: internal tld: S.LAN",
		"a.md: internal tld: //db.internal",
	}
	if hits := scanFixed(t, leaky); !slices.Equal(hits, want) {
		t.Fatalf("scanFixed(leaky) = %q, want %q", hits, want)
	}
	all := []fs.FS{profiles.Core(), profiles.Builtin(), profiles.Features()}
	for _, fsys := range all {
		if hits := scanFixed(t, fsys); len(hits) > 0 {
			t.Errorf("fixed denylist: %v", hits)
		}
	}
	words := privateWords(t)
	for _, fsys := range all {
		if hits := profiles.Scan(fsys, words); len(hits) > 0 {
			t.Errorf("private denylist: %v", hits)
		}
	}
}

// privateWords reads the gitignored .private/denylist.txt, one term a line, # for comments; it
// skips the test when the file is absent (CI), so the private scan runs only where it exists.
func privateWords(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../.private/denylist.txt")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip(".private/denylist.txt is absent (CI): the private scan is skipped")
	}
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			words = append(words, l)
		}
	}
	return words
}

// publicTree loads the repository into memory, binaries included, without what is never
// published: the private docs/discovery and .private, git's own data, dependencies, the site's
// build output and session worktrees. Symlinks are skipped; their targets are read in place.
func publicTree(t *testing.T, repo fs.FS) fstest.MapFS {
	t.Helper()
	skip := map[string]bool{
		"docs/discovery": true, ".private": true, ".git": true, ".worktrees": true,
		".claude/worktrees": true, "site/.next": true, "site/out": true, "site/.source": true,
	}
	tree := fstest.MapFS{}
	err := fs.WalkDir(repo, ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case skip[p] || d.Name() == "node_modules":
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		case d.IsDir() || d.Type()&fs.ModeSymlink != 0:
			return nil
		}
		b, rerr := fs.ReadFile(repo, p)
		tree[p] = &fstest.MapFile{Data: b}
		return rerr
	})
	if err != nil {
		t.Fatalf("publicTree: %v", err)
	}
	return tree
}

func TestPublicTreeSelection(t *testing.T) {
	// The scanned set is the whole repository, binaries, agent texts and testdata included,
	// and never the private folders, git data, dependencies, site build output or worktrees.
	// covers: R-QA-048
	repo := fstest.MapFS{
		"docs/index.md":                 {Data: []byte("x")},
		"docs/discovery/notes.md":       {Data: []byte("x")},
		"docs/discovery-public.md":      {Data: []byte("x")},
		"README.md":                     {Data: []byte("x")},
		"AGENTS.md":                     {Data: []byte("x")},
		"CLAUDE.md":                     {Data: []byte("x"), Mode: fs.ModeSymlink},
		".agents/skills/a/SKILL.md":     {Data: []byte("x")},
		".claude/settings.json":         {Data: []byte("x")},
		".claude/worktrees/w/AGENTS.md": {Data: []byte("x")},
		".github/workflows/ci.yml":      {Data: []byte("x")},
		".git/HEAD":                     {Data: []byte("x")},
		".private/denylist.txt":         {Data: []byte("x")},
		".worktrees/s/README.md":        {Data: []byte("x")},
		"evals/a/eval.py":               {Data: []byte("x")},
		"internal/x/testdata/t.txtar":   {Data: []byte("x")},
		"dist/ness":                     {Data: []byte{0, 1, 2}},
		"site/app/page.tsx":             {Data: []byte("x")},
		"site/out/index.html":           {Data: []byte("x")},
		"site/.next/cache":              {Data: []byte("x")},
		"site/.source/index.ts":         {Data: []byte("x")},
		"site/node_modules/a/index.js":  {Data: []byte("x")},
		"site/app/node_modules/b.js":    {Data: []byte("x")},
	}
	var got []string
	for p := range publicTree(t, repo) {
		got = append(got, p)
	}
	slices.Sort(got)
	want := []string{
		".agents/skills/a/SKILL.md", ".claude/settings.json", ".github/workflows/ci.yml",
		"AGENTS.md", "README.md", "dist/ness", "docs/discovery-public.md", "docs/index.md",
		"evals/a/eval.py", "internal/x/testdata/t.txtar", "site/app/page.tsx",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("publicTree = %v, want %v", got, want)
	}
}

func TestPublicTreeDenylist(t *testing.T) {
	// The repository names none of the private terms (R-BLT-003); skipped without the private
	// list, like the profile scan.
	// covers: R-QA-048
	words := privateWords(t)
	if hits := profiles.Scan(publicTree(t, os.DirFS("..")), words); len(hits) > 0 {
		t.Errorf("private denylist in the public tree: %v", hits)
	}
}
