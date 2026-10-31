package sync_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

func file(path, content string) nsync.Entry {
	return nsync.Entry{Path: path, Kind: nsync.KindFile, Owner: "p", Content: []byte(content)}
}

func blockEntry(content string) nsync.Entry {
	return nsync.Entry{Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "p", Name: "p/x", Content: []byte(content)}
}

func syncOnce(t *testing.T, root *os.Root, entries ...nsync.Entry) *nsync.Plan {
	t.Helper()
	p, err := nsync.Compute(root, entries, nsync.LockMeta{Ness: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := nsync.Apply(root, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, root *os.Root, path string) (string, bool) {
	t.Helper()
	b, err := root.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func lockEntry(t *testing.T, root *os.Root, id string) *nsync.LockEntry {
	t.Helper()
	b, ok := read(t, root, nsync.LockPath)
	if !ok {
		return nil
	}
	l, err := nsync.ParseLock([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	for i := range l.Entries {
		if l.Entries[i].ID == id {
			return &l.Entries[i]
		}
	}
	return nil
}

func TestLockHashIsSHA256(t *testing.T) {
	// covers: R-SYNC-087, R-SYNC-010
	root := openRoot(t)
	syncOnce(t, root, file("a.txt", "hello\n"), blockEntry("inside"))
	h := sha256.Sum256([]byte("hello\n"))
	if e := lockEntry(t, root, "a.txt"); e == nil || e.Hash != hex.EncodeToString(h[:]) || e.Owner != "p" {
		t.Fatalf("lock entry %+v", e)
	}
	hb := sha256.Sum256([]byte("inside\n"))
	if e := lockEntry(t, root, "AGENTS.md#p/x"); e == nil || e.Hash != hex.EncodeToString(hb[:]) {
		t.Fatalf("block entry %+v", e)
	}
}

func TestOrphanOfKeptVersionIsKept(t *testing.T) {
	// covers: R-SYNC-020
	root := openRoot(t)
	if err := root.WriteFile("a.txt", []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := nsync.Compute(root, []nsync.Entry{file("a.txt", "theirs\n")}, nsync.LockMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Keep("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := nsync.Apply(root, p); err != nil {
		t.Fatal(err)
	}
	syncOnce(t, root) // a.txt no longer rendered
	if got, ok := read(t, root, "a.txt"); !ok || got != "mine\n" {
		t.Fatalf("kept file deleted or changed: %q %v", got, ok)
	}
}

func TestKeepRefusesCleanPath(t *testing.T) {
	// covers: R-SYNC-018
	root := openRoot(t)
	p, err := nsync.Compute(root, []nsync.Entry{file("a.txt", "x\n")}, nsync.LockMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Keep("a.txt"); err == nil || !strings.Contains(err.Error(), "not in conflict") {
		t.Fatalf("Keep on a clean path: %v", err)
	}
}

// FuzzSyncNeverClobbers runs random sequences of user edits, upstream changes and syncs, and
// checks after each sync that ness changed only bytes it wrote and nobody touched since.
func FuzzSyncNeverClobbers(f *testing.F) {
	// covers: R-SYNC-021, R-SYNC-004, R-SYNC-044
	f.Add([]byte{3, 0, 3, 1, 3, 4, 3, 2, 3})
	f.Add([]byte{5, 3, 6, 1, 3, 4, 1, 3, 2, 3})
	f.Add([]byte{3, 6, 1, 12, 3, 1, 3}) // block conflict, then sync --keep AGENTS.md
	f.Fuzz(func(t *testing.T, ops []byte) {
		root := openRoot(t)
		version, rendered := 0, true
		for i, op := range ops {
			switch op % 8 {
			case 0:
				_ = root.WriteFile("a.txt", []byte(fmt.Sprintf("user %d\n", i)), 0o600)
			case 1:
				version++
			case 2:
				rendered = !rendered
			case 3:
				checkedSync(t, root, version, rendered, "")
			case 4: // sync --keep a.txt, or AGENTS.md when bit 3 is set (seeds stay valid)
				checkedSync(t, root, version, rendered, []string{"a.txt", "AGENTS.md"}[op/8%2])
			case 5:
				old, _ := read(t, root, "AGENTS.md")
				_ = root.WriteFile("AGENTS.md", []byte(fmt.Sprintf("line %d\n%s", i, old)), 0o600)
			case 6:
				old, _ := read(t, root, "AGENTS.md")
				_ = root.WriteFile("AGENTS.md", []byte(strings.Replace(old, "\nb", "\nuser b", 1)), 0o600)
			case 7: // the user moves a.txt to mine.txt and links a.txt to it
				old, _ := read(t, root, "a.txt")
				_ = root.Remove("a.txt")
				_ = root.WriteFile("mine.txt", []byte(old), 0o600)
				_ = root.Symlink("mine.txt", "a.txt")
			}
		}
	})
}

func desired(version int, rendered bool) []nsync.Entry {
	if !rendered {
		return nil
	}
	return []nsync.Entry{file("a.txt", fmt.Sprintf("v%d\n", version)), blockEntry(fmt.Sprintf("b%d\n", version))}
}

// checkedSync syncs (after sync --keep keep when keep is set; a refused keep writes nothing)
// and checks D4: a.txt and the block's content change only when ness wrote them and nobody
// touched them since, and text outside the block never changes.
func checkedSync(t *testing.T, root *os.Root, version int, rendered bool, keep string) {
	t.Helper()
	preA, hadA := read(t, root, "a.txt")
	preAgents, _ := read(t, root, "AGENTS.md")
	preMine, hadMine := read(t, root, "mine.txt")
	recA, recB := lockEntry(t, root, "a.txt"), lockEntry(t, root, "AGENTS.md#p/x")
	p, err := nsync.Compute(root, desired(version, rendered), nsync.LockMeta{Ness: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if keep != "" && p.Keep(keep) != nil {
		return
	}
	if err := nsync.Apply(root, p); err != nil {
		t.Fatal(err)
	}
	postA, hasA := read(t, root, "a.txt")
	if hadA && (!hasA || postA != preA) && !nessWrote(recA, preA) {
		t.Fatalf("a.txt %q was changed to %q (present %v) although ness had not written it; lock %+v", preA, postA, hasA, recA)
	}
	if postMine, hasMine := read(t, root, "mine.txt"); hadMine && (!hasMine || postMine != preMine) {
		t.Fatalf("mine.txt, never rendered, was changed from %q to %q (present %v)", preMine, postMine, hasMine)
	}
	postAgents, _ := read(t, root, "AGENTS.md")
	if outside(preAgents) != outside(postAgents) {
		t.Fatalf("text outside the block changed:\n%q\n%q", preAgents, postAgents)
	}
	preB, hadB := inside(preAgents)
	postB, hasB := inside(postAgents)
	if hadB && (!hasB || postB != preB) && !nessWrote(recB, preB) {
		t.Fatalf("block %q was changed to %q (present %v) although ness had not written it; lock %+v", preB, postB, hasB, recB)
	}
}

func nessWrote(rec *nsync.LockEntry, content string) bool {
	h := sha256.Sum256([]byte(content))
	return rec != nil && rec.Hash == rec.Upstream && rec.Hash == hex.EncodeToString(h[:])
}

// outside strips the p/x block (markers included) and the blank line sync puts before it.
func outside(s string) string {
	const endMarker = "<!-- ness:end p/x -->\n"
	begin, end := strings.Index(s, "<!-- ness:begin p/x -->"), strings.Index(s, endMarker)
	if begin >= 0 && end > begin {
		head := strings.TrimRight(s[:begin], "\n")
		if head != "" {
			head += "\n"
		}
		s = head + s[end+len(endMarker):]
	}
	return strings.TrimRight(s, "\n")
}

// inside returns the p/x block's content between its markers.
func inside(s string) (string, bool) {
	_, rest, ok := strings.Cut(s, "<!-- ness:begin p/x -->\n")
	if !ok {
		return "", false
	}
	inner, _, ok := strings.Cut(rest, "<!-- ness:end p/x -->\n")
	return inner, ok
}

func planFor(p *nsync.Plan, path string) *nsync.FilePlan {
	for i := range p.Files {
		if p.Files[i].Path == path {
			return &p.Files[i]
		}
	}
	return nil
}

func TestUnparsablePartFileIsLeftAlone(t *testing.T) {
	// covers: R-SYNC-055, R-SYNC-004
	root := openRoot(t)
	key := nsync.Entry{Path: "s.json", Kind: nsync.KindKey, Owner: "p", Name: "a", Content: []byte("1")}
	syncOnce(t, root, key)
	before := lockEntry(t, root, "s.json:a")
	if err := root.WriteFile("s.json", []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	key.Content = []byte("2")
	p := syncOnce(t, root, key)
	if fp := planFor(p, "s.json"); fp == nil || fp.Err == nil || !strings.Contains(fp.Err.Error(), "s.json") {
		t.Fatalf("want an error naming s.json, got %+v", fp)
	}
	if got, _ := read(t, root, "s.json"); got != "not json" {
		t.Fatalf("unparsable file changed: %q", got)
	}
	if after := lockEntry(t, root, "s.json:a"); after == nil || *after != *before {
		t.Fatalf("lock entry not preserved: %+v, was %+v", after, before)
	}
}

func TestNonObjectAncestorHoldsTheFile(t *testing.T) {
	// covers: R-SYNC-050, R-SYNC-055
	root := openRoot(t)
	if err := root.WriteFile("s.json", []byte(`{"env": "x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	key := nsync.Entry{Path: "s.json", Kind: nsync.KindKey, Owner: "p", Name: "env.M", Content: []byte(`"m"`)}
	p := syncOnce(t, root, key)
	if fp := planFor(p, "s.json"); fp == nil || fp.Err == nil || !strings.Contains(fp.Err.Error(), "env") {
		t.Fatalf("want an error naming env, got %+v", fp)
	}
	if got, _ := read(t, root, "s.json"); got != `{"env": "x"}` {
		t.Fatalf("file changed: %q", got)
	}
}

func TestFailedFileKeepsEachLockEntryOnce(t *testing.T) {
	// covers: R-SYNC-071
	root := openRoot(t)
	x := nsync.Entry{Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "p", Name: "p/x", Content: []byte("x\n")}
	y := nsync.Entry{Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "p", Name: "p/y", Content: []byte("y\n")}
	syncOnce(t, root, x, y)
	x.Content = []byte("<!-- ness:begin p/z -->\n") // makes the render's markers malformed
	p, err := nsync.Compute(root, []nsync.Entry{x, y}, nsync.LockMeta{Ness: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if fp := planFor(p, "AGENTS.md"); fp == nil || fp.Err == nil {
		t.Fatalf("want a planning error for AGENTS.md, got %+v", fp)
	}
	b, err := p.LockBytes()
	if err != nil {
		t.Fatal(err)
	}
	l, err := nsync.ParseLock(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 2 {
		t.Fatalf("want the 2 recorded entries once each, got %+v", l.Entries)
	}
}

func TestConflictDoesNotWriteThroughNessNewLink(t *testing.T) {
	// covers: R-SYNC-004, R-SYNC-012
	root := openRoot(t)
	for name, content := range map[string]string{"a.txt": "user\n", "mine.txt": "mine\n"} {
		if err := root.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := root.Symlink("mine.txt", "a.txt.ness-new"); err != nil {
		t.Fatal(err)
	}
	syncOnce(t, root, file("a.txt", "theirs\n"))
	if got, _ := read(t, root, "mine.txt"); got != "mine\n" {
		t.Fatalf("mine.txt written through the .ness-new link: %q", got)
	}
	if got, _ := read(t, root, "a.txt.ness-new"); got != "theirs\n" {
		t.Fatalf(".ness-new holds %q", got)
	}
}

func TestKeepRetainsRecordOfPlannedBlockDelete(t *testing.T) {
	// covers: R-SYNC-017, R-SYNC-071
	root := openRoot(t)
	x := nsync.Entry{Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "p", Name: "p/x", Content: []byte("x\n")}
	y := nsync.Entry{Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "p", Name: "p/y", Content: []byte("y\n")}
	syncOnce(t, root, x, y)
	recY := lockEntry(t, root, "AGENTS.md#p/y")
	old, _ := read(t, root, "AGENTS.md")
	if err := root.WriteFile("AGENTS.md", []byte(strings.Replace(old, "x\n", "user x\n", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	x.Content = []byte("x2\n") // p/x conflicts; p/y is no longer rendered, so its delete is planned
	p, err := nsync.Compute(root, []nsync.Entry{x}, nsync.LockMeta{Ness: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Keep("AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	if err := nsync.Apply(root, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := read(t, root, "AGENTS.md"); !strings.Contains(got, "<!-- ness:begin p/y -->") {
		t.Fatalf("keep changed the file: %q", got)
	}
	if after := lockEntry(t, root, "AGENTS.md#p/y"); after == nil || *after != *recY {
		t.Fatalf("p/y, still in the kept file, lost its record: %+v, was %+v", after, recY)
	}
	syncOnce(t, root, x) // the next sync removes the ness-written block
	if got, _ := read(t, root, "AGENTS.md"); strings.Contains(got, "p/y") {
		t.Fatalf("p/y not removed after keep: %q", got)
	}
}
