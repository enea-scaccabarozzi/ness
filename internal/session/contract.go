package session

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
)

const (
	// LockFile is the contract lock in a session's record directory (R-CTR-001).
	LockFile = "contract.lock"
	// Amended marks a frozen file open for amendment in the lock (R-CTR-005).
	Amended = "amend"
)

// ErrBadLock is returned for a contract.lock that does not parse; the message names it.
var ErrBadLock = errors.New("malformed contract.lock")

var (
	lockLine = regexp.MustCompile(`^([0-9a-f]{40}|amend|scn:[0-9a-f]{64}|prefix:\d+:[0-9a-f]{64}|` +
		`fmt:[a-z][a-z0-9-]*:[0-9a-f]{64}) (\S.*)$`)
	// scenarioLine is one locked scenario of a "scn:" file: "scenario <hash> <path>\t<name>".
	scenarioLine = regexp.MustCompile(`^scenario ([0-9a-f]{64}) ([^\t]+)\t(.+)$`)
)

// Lock is one contract.lock: Dir is its repo-relative record directory, Files maps a
// repo-relative path to its git blob sha, a "scn:" or "prefix:" token (R-CTR-031, R-CTR-033), or
// Amended while it is open for change. Scenarios holds, for each "scn:" path, its locked
// scenario names and hashes.
type Lock struct {
	Dir       string
	Files     map[string]string
	Scenarios map[string]map[string]string
}

// ParseLock reads "<sha> <path>", "<token> <path>", "amend <path>" and
// "scenario <hash> <path>\t<name>" lines; blank and # lines are skipped. A "scn:" entry whose
// scenario lines do not match its digest is malformed: a lock cannot be loosened by hand.
func ParseLock(data []byte) (Lock, error) {
	l := Lock{Files: map[string]string{}, Scenarios: map[string]map[string]string{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		if line := sc.Text(); line != "" && !strings.HasPrefix(line, "#") && !l.add(line) {
			return Lock{}, fmt.Errorf("%w: line %d: %q is not \"<sha> <path>\" or \"amend <path>\"",
				ErrBadLock, n, line)
		}
	}
	if p := l.badScenarios(); p != "" {
		return Lock{}, fmt.Errorf("%w: the scenario or unit lines of %s do not match its entry", ErrBadLock, p)
	}
	return l, sc.Err()
}

// unitLocked reports whether the token's file keeps scenario lines: a "scn:" or "fmt:" entry.
func unitLocked(tok string) bool {
	return strings.HasPrefix(tok, ScenarioTag) || strings.HasPrefix(tok, FormatTag)
}

// badScenarios names a path whose scenario lines and "scn:" or "fmt:" entry disagree (a line with
// no entry, an entry with other lines), or "".
func (l Lock) badScenarios() string {
	for p, tok := range l.Files {
		if unitLocked(tok) && !strings.HasSuffix(tok, ":"+scenarioDigest(l.Scenarios[p])) {
			return p
		}
	}
	for p := range l.Scenarios {
		if !unitLocked(l.Files[p]) {
			return p
		}
	}
	return ""
}

// add records one lock line, reporting whether it is a known form.
func (l Lock) add(line string) bool {
	if m := scenarioLine.FindStringSubmatch(line); m != nil {
		if l.Scenarios[m[2]] == nil {
			l.Scenarios[m[2]] = map[string]string{}
		}
		l.Scenarios[m[2]][m[3]] = m[1]
		return true
	}
	m := lockLine.FindStringSubmatch(line)
	if m != nil {
		l.Files[m[2]] = m[1]
	}
	return m != nil
}

// Bytes renders the lock sorted by path, then the scenario lines.
func (l Lock) Bytes() []byte {
	var b bytes.Buffer
	for _, p := range slices.Sorted(maps.Keys(l.Files)) {
		fmt.Fprintf(&b, "%s %s\n", l.Files[p], p)
	}
	for _, p := range slices.Sorted(maps.Keys(l.Scenarios)) {
		for _, n := range slices.Sorted(maps.Keys(l.Scenarios[p])) {
			fmt.Fprintf(&b, "scenario %s %s\t%s\n", l.Scenarios[p][n], p, n)
		}
	}
	return b.Bytes()
}

// ReadLock reads <root>/<dir>/contract.lock; a missing file is an empty lock.
func ReadLock(root, dir string) (Lock, error) {
	l := Lock{Dir: dir, Files: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), LockFile)) //nolint:gosec // under the repo
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	parsed, err := ParseLock(data)
	if err != nil {
		return l, fmt.Errorf("%s/%s: %w", dir, LockFile, err)
	}
	parsed.Dir = dir
	return parsed, nil
}

// WriteLock writes the lock into its record directory, creating the directory; the file appears
// whole or not at all (temp file and rename).
func WriteLock(root string, l Lock) error {
	dir := filepath.Join(root, filepath.FromSlash(l.Dir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".contract.lock.*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(l.Bytes())
	if err = errors.Join(err, tmp.Close()); err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(dir, LockFile))
	}
	if err != nil {
		err = errors.Join(err, os.Remove(tmp.Name()))
	}
	return err
}

// FindLocks returns every contract.lock under <root>/<sessions>, sorted by directory. A malformed
// one is an error.
func FindLocks(root, sessions string) ([]Lock, error) {
	locks, _, err := findLocks(root, sessions, func(string) bool { return false })
	return locks, err
}

// FindLocksTolerant is FindLocks for the gate (R-CTR-044): a malformed lock whose file is
// identical at ref (the merge-base, R-CTR-046) predates this branch, as a JSON lock of an old harness does;
// it is skipped and its path returned. One the branch adds or modifies still fails closed.
func FindLocksTolerant(g Git, root, sessions, ref string) (locks []Lock, skipped []string, err error) {
	return findLocks(root, sessions, func(rel string) bool {
		want, err := g.BaseBlob(root, ref, rel)
		if err != nil {
			return false // not at ref: added on the branch
		}
		got, err := g.BlobSha(root, rel)
		return err == nil && got == want
	})
}

// findLocks walks the sessions dir; legacy decides, for a lock that does not parse, whether it
// is skipped (its path is then returned) instead of failing the walk.
func findLocks(root, sessions string, legacy func(rel string) bool) (locks []Lock, skipped []string, err error) {
	base := filepath.Join(root, filepath.FromSlash(sessions))
	err = filepath.WalkDir(base, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() || d.Name() != LockFile {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(p))
		if rerr != nil {
			return rerr
		}
		l, lerr := ReadLock(root, filepath.ToSlash(rel))
		if lerr != nil {
			if lp := filepath.ToSlash(rel) + "/" + LockFile; errors.Is(lerr, ErrBadLock) && legacy(lp) {
				skipped = append(skipped, lp)
				return nil
			}
			return lerr
		}
		locks = append(locks, l)
		return nil
	})
	sort.Slice(locks, func(i, j int) bool { return locks[i].Dir < locks[j].Dir })
	return locks, skipped, err
}

// Frozen is one frozen path: its recorded sha (or Amended) and the lock holding it; LockDir ""
// is a project-wide path from [contracts] frozen, compared with its blob at base.
type Frozen struct {
	Path, Sha, LockDir string
	Scenarios          map[string]string // the locked scenarios of a "scn:" entry
}

// FrozenFiles joins the locks. A path in several locks takes the first lock by Dir, except that
// an open amendment anywhere opens the path. The project-wide paths (ProjectFrozen) are not part
// of it: they are checked on their own, so no lock entry hides or opens one (R-CTR-045).
func FrozenFiles(locks []Lock) map[string]Frozen {
	out := map[string]Frozen{}
	for _, l := range locks {
		for p, sha := range l.Files {
			cur, seen := out[p]
			if !seen || (sha == Amended && cur.Sha != Amended) {
				out[p] = Frozen{Path: p, Sha: sha, LockDir: l.Dir, Scenarios: l.Scenarios[p]}
			}
		}
	}
	return out
}

// BlobSha is git hash-object of the working-tree file.
func (g Git) BlobSha(dir, path string) (string, error) { return g.Out(dir, "hash-object", "--", path) }

// BaseBlob is the sha of path's blob at base.
func (g Git) BaseBlob(dir, base, path string) (string, error) {
	return g.Out(dir, "rev-parse", "--verify", "--quiet", base+":"+path)
}

// CheckContracts returns the frozen paths that no longer hold (amended ones skipped): a
// whole-file path whose working-tree blob differs from the recorded sha or, for a project-wide
// path, from its blob at base; a scenario-locked file with a locked scenario edited or removed; a
// prefix-locked file whose locked words are no longer its start; a format-locked file with a locked
// unit changed or removed. A missing file differs (R-CTR-003, R-CTR-031, R-CTR-033, R-CTR-050).
// It reads, and runs the units commands of c through x.
func CheckContracts(
	g Git, root, base string, frozen map[string]Frozen, c config.Contracts, x *Extractor,
) ([]Diff, error) {
	var diff []Diff
	for _, p := range slices.Sorted(maps.Keys(frozen)) {
		f := frozen[p]
		if f.Sha == Amended {
			continue
		}
		if Lenient(f.Sha) {
			if what := lenientDiff(root, f, c, x); what != "" {
				diff = append(diff, Diff{p, what})
			}
			continue
		}
		want := f.Sha
		if f.LockDir == "" {
			var err error
			if want, err = g.BaseBlob(root, base, p); err != nil {
				return nil, fmt.Errorf("%s: not in %s: %w", p, base, err)
			}
		}
		got, err := g.BlobSha(root, p)
		if err != nil || got != want { // an unreadable file is a changed file
			diff = append(diff, Diff{Path: p})
		}
	}
	return diff, nil
}

// MergeBase is the merge-base of base and HEAD.
func MergeBase(g Git, root, base string) (string, error) {
	mb, err := g.Out(root, "merge-base", base, "HEAD")
	if err != nil {
		return "", fmt.Errorf("merge-base %s HEAD: %w", base, err)
	}
	return mb, nil
}

// HistoricalFrozen is every path any contract.lock under sessions held at the merge-base mb, in
// any commit of mb..HEAD, or in the working tree. Each maps to the lock dir that first held it
// and its reference sha: the sha the merge-base recorded or, for a path first frozen on the
// branch, the sha of the commit that first froze it (the working tree's for one not committed
// yet); "" when no sha was ever recorded. Any git error is returned: the caller fails closed.
func HistoricalFrozen(g Git, root, sessions, mb string, have []Lock) (map[string]Frozen, error) {
	revs, err := g.Out(root, "rev-list", "--topo-order", "--reverse", mb+"..HEAD")
	if err != nil {
		return nil, fmt.Errorf("rev-list %s..HEAD: %w", mb, err)
	}
	out := map[string]Frozen{}
	seen := map[string]bool{} // blob shas already read
	for _, ref := range append([]string{mb}, strings.Fields(revs)...) {
		if lerr := lockPathsAt(g, root, sessions, ref, seen, out); lerr != nil {
			return nil, lerr
		}
	}
	for _, l := range have {
		addRef(out, l.Dir, l.Files)
	}
	return out, nil
}

// addRef records files of the lock in dir: the first lock to hold a path names it, the first sha
// recorded for it is its reference.
func addRef(out map[string]Frozen, dir string, files map[string]string) {
	for p, sha := range files {
		f, ok := out[p]
		if !ok {
			f = Frozen{Path: p, LockDir: dir}
		}
		if f.Sha == "" && sha != Amended {
			f.Sha = sha
		}
		out[p] = f
	}
}

// lockEntry splits one ls-tree entry into blob sha and path, ok for a contract.lock.
func lockEntry(ent string) (sha, path string, ok bool) {
	meta, path, ok := strings.Cut(ent, "\t")
	f := strings.Fields(meta)
	if !ok || len(f) != 3 || filepath.Base(path) != LockFile {
		return "", "", false
	}
	return f[2], path, true
}

// lockBlob parses the contract.lock stored as git blob sha.
func lockBlob(g Git, root, sha string) (Lock, error) {
	blob, err := g.Out(root, "cat-file", "blob", sha)
	if err != nil {
		return Lock{}, err
	}
	return ParseLock([]byte(blob))
}

// lockPathsAt adds to out every path the contract.locks of commit ref hold; seen skips a blob
// already read. A malformed blob, at the merge-base or in a branch commit, is marked seen too and
// skipped without an error: it holds no paths, and the worktree walk (FindLocksTolerant) still
// fails on a copy that exists now.
func lockPathsAt(g Git, root, sessions, ref string, seen map[string]bool, out map[string]Frozen) error {
	tree, err := g.Out(root, "ls-tree", "-r", "-z", ref, "--", sessions)
	if err != nil {
		return fmt.Errorf("ls-tree %s: %w", ref, err)
	}
	for _, ent := range strings.Split(tree, "\x00") {
		sha, path, ok := lockEntry(ent)
		if !ok || seen[sha] {
			continue
		}
		seen[sha] = true
		l, perr := lockBlob(g, root, sha)
		if perr != nil && errors.Is(perr, ErrBadLock) {
			continue // a lock version that does not parse holds no paths; the worktree walk judges the ones present
		}
		if perr != nil {
			return fmt.Errorf("%s at %s: %w", path, ref, perr)
		}
		addRef(out, filepath.ToSlash(filepath.Dir(path)), l.Files)
	}
	return nil
}

// Unamended returns the lock paths whose working-tree blob differs from their reference sha in
// hist without an "Amend <path>:" line (what ness contract amend writes) added since mb to the
// decisions.md of the lock holding them. The lock itself is not trusted: a sha or an amend
// written into it by Bash is no decision. Paths in skip are already reported.
func Unamended(g Git, root, mb string, frozen, hist map[string]Frozen, skip []string) ([]string, error) {
	var out []string
	for _, p := range slices.Sorted(maps.Keys(frozen)) {
		f, ref := frozen[p], hist[p].Sha
		if f.LockDir == "" || ref == "" || slices.Contains(skip, p) {
			continue
		}
		if unchanged(g, root, f, ref) {
			continue
		}
		ok, err := amendLogged(g, root, mb, f.LockDir, p)
		if err != nil {
			return nil, err
		}
		if !ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// unchanged reports whether f still holds its reference: the working-tree blob for a whole-file
// lock, the lock's own token for a scenario or prefix lock (the tree may legally grow).
func unchanged(g Git, root string, f Frozen, ref string) bool {
	if Lenient(f.Sha) {
		return f.Sha == ref
	}
	got, err := g.BlobSha(root, f.Path)
	return err == nil && got == ref
}

// amendLogged reports whether dir/decisions.md holds more "Amend <p>:" lines now than at mb.
func amendLogged(g Git, root, mb, dir, p string) (bool, error) {
	rel := dir + "/decisions.md"
	now, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) //nolint:gosec // under the repo
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	var then string
	if ent, lerr := g.Out(root, "ls-tree", mb, "--", rel); lerr != nil {
		return false, fmt.Errorf("ls-tree %s: %w", mb, lerr)
	} else if ent != "" {
		if then, err = g.Out(root, "cat-file", "blob", mb+":"+rel); err != nil {
			return false, fmt.Errorf("%s at %s: %w", rel, mb, err)
		}
	}
	marker := "Amend " + p + ": "
	return amendLines(string(now), marker) > amendLines(then, marker), nil
}

// amendLines counts the lines of decisions text that start with marker.
func amendLines(text, marker string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, marker) {
			n++
		}
	}
	return n
}

// ProjectAmended reports whether the decisions.md files under sessions hold more
// "Amend <p> (project-wide): " lines (what ness contract amend writes for a project-wide path), all
// together, now than at mb (R-CTR-056). Totals, not per file: a record directory moved or copied
// opens nothing. Like R-CTR-021 it is a record, not a security boundary.
func ProjectAmended(g Git, root, sessions, mb, p string) (bool, error) {
	marker := "Amend " + p + " (project-wide): "
	now := 0
	dir := filepath.Join(root, filepath.FromSlash(sessions))
	err := filepath.WalkDir(dir, func(name string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() || de.Name() != "decisions.md" {
			return err
		}
		b, rerr := os.ReadFile(name) //nolint:gosec // under the repo
		now += amendLines(string(b), marker)
		return rerr
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if now == 0 {
		return false, nil
	}
	spec := ":(glob)" + strings.TrimPrefix(strings.TrimSuffix(filepath.ToSlash(sessions), "/")+"/**/decisions.md", "/")
	then, err := g.Out(root, "grep", "-h", "-F", "-e", marker, mb, "--", spec)
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) && ee.ExitCode() == 1 {
		then, err = "", nil // no match at mb
	}
	if err != nil {
		return false, fmt.Errorf("grep %s: %w", mb, err)
	}
	return now > amendLines(then, marker), nil
}

// Guard is the pre-tool contract guard (R-CTR-009..012): it resolves the edited file through
// its own worktree, never through CLAUDE_PROJECT_DIR (R-CTR-010), and compares the real path.
// An unreadable lock is reported on Warn and the edit allowed: the gate's check fails closed.
type Guard struct {
	Git      Git
	Sessions string
	Project  []string         // [contracts] frozen entries, globs allowed
	Prefix   []string         // [contracts] prefix globs
	Formats  config.Contracts // [contracts]; its formats decide which files are format-locked
	Base     string           // the base branch; its merge-base with HEAD is the reference (R-CTR-046)
	Warn     io.Writer
}

// Refuse returns why the edit of file (relative to cwd when relative) is refused, or ""; a
// scenario or prefix lock refuses nothing (see Verdict).
func (gd Guard) Refuse(file, cwd string) string {
	if reason, soft := gd.Verdict(file, cwd); !soft {
		return reason
	}
	return ""
}

// Verdict returns what the guard says of editing file: a reason, and soft when the file is
// scenario- or prefix-locked, where additions are legal and the gate enforces (R-CTR-036), so the
// reason is a warning to show while the edit proceeds.
func (gd Guard) Verdict(file, cwd string) (reason string, soft bool) {
	if file == "" {
		return "", false
	}
	if !filepath.IsAbs(file) {
		// Not filepath.Join: its lexical clean would fold link/.. before the symlink resolves.
		file = cwd + string(filepath.Separator) + file
	}
	resolved := RealFile(file)
	root, rel, ok := gd.repoPath(resolved)
	if !ok {
		return "", false // not in a repository: nothing is frozen there
	}
	// A session lock is changed only by ness contract; a contract.lock outside the sessions dir
	// is an ordinary file.
	sessions := strings.TrimSuffix(filepath.ToSlash(gd.Sessions), "/") + "/"
	if filepath.Base(rel) == LockFile && strings.HasPrefix(rel, sessions) {
		return rel + " is the contract lock; change it with ness contract, never by editing it", false
	}
	return gd.frozenVerdict(root, rel)
}

// frozenVerdict is Verdict for a file that is no lock: the locks and the project-wide entry.
func (gd Guard) frozenVerdict(root, rel string) (reason string, soft bool) {
	mb := ""
	if gd.Base != "" {
		var merr error
		if mb, merr = MergeBase(gd.Git, root, gd.Base); merr != nil {
			return gd.allow(merr)
		}
	}
	locks, err := gd.locks(root, mb)
	if err != nil {
		return gd.allow(err)
	}
	// The project-wide entry is judged on its own: a lock entry for the path, an open amendment
	// included, never replaces it (R-CTR-045). The stricter verdict wins; a soft one only when
	// nothing refuses.
	pf, inProject := gd.projectEntry(root, mb, rel)
	if inProject && !Lenient(pf.Sha) {
		return frozenMessage(pf)
	}
	if f, ok := FrozenFiles(locks)[rel]; ok && f.Sha != Amended {
		return frozenMessage(f)
	}
	if inProject {
		return frozenMessage(pf)
	}
	return "", false
}

// allow reports err on Warn and allows the edit: the gate's check fails closed.
func (gd Guard) allow(err error) (string, bool) {
	//nolint:errcheck // a failed warning changes nothing
	fmt.Fprintf(gd.Warn, "ness: contract guard: %v (edits allowed until it is fixed)\n", err)
	return "", false
}

// locks is FindLocks, tolerant of a legacy lock identical at the merge-base mb when Base is set.
func (gd Guard) locks(root, mb string) ([]Lock, error) {
	if mb == "" {
		return FindLocks(root, gd.Sessions)
	}
	locks, _, err := FindLocksTolerant(gd.Git, root, gd.Sessions, mb)
	return locks, err
}

// projectEntry is rel as a project-wide frozen path, in the mode check gives it (projectEntry,
// R-CTR-043); a glob entry freezes only a file that exists at the merge-base mb. A path amended
// project-wide is open (R-CTR-056); an unreadable record is reported on Warn and opens it too.
func (gd Guard) projectEntry(root, mb, rel string) (Frozen, bool) {
	if !slices.Contains(gd.Project, rel) {
		if _, err := gd.Git.BaseBlob(root, mb, rel); err != nil || !MatchAny(gd.Project, rel) {
			return Frozen{}, false
		}
	}
	if mb != "" {
		if amended, err := ProjectAmended(gd.Git, root, gd.Sessions, mb, rel); err != nil || amended {
			if err != nil {
				gd.allow(err)
			}
			return Frozen{}, false
		}
	}
	if name, _, ok := gd.Formats.FormatOf(rel); ok {
		// The guard runs no units command: a format file is soft whatever its units are.
		return Frozen{Path: rel, Sha: FormatTag + name + ":"}, true
	}
	return projectEntry(gd.Git, root, mb, rel, gd.Prefix), true
}

// frozenMessage is what the guard says of editing the frozen file f; soft for a scenario or
// prefix lock (R-CTR-036).
func frozenMessage(f Frozen) (reason string, soft bool) {
	route := "ask your manager (ness send --to manager ...), then run ness contract amend " + f.Path +
		" --reason TEXT --item ITEM" + rule.Ref(rule.ContractGuard)
	switch {
	case f.LockDir == "" && Lenient(f.Sha):
		return f.Path + " is locked project-wide ([contracts] frozen in .ness/ness.toml): adding is fine, but " +
			"editing or removing what was locked at the base branch fails ness contract check; " +
			"to change locked text, amend it on the base branch" + rule.Ref(rule.ContractGuard), true
	case f.LockDir == "":
		return f.Path + " is frozen project-wide ([contracts] frozen in .ness/ness.toml);" +
			" change it on the base branch" + rule.Ref(rule.ContractGuard), false
	case Lenient(f.Sha):
		return fmt.Sprintf("%s is locked (%s/%s): adding is fine, but editing or removing what was locked fails "+
			"ness contract check; to change locked text, %s", f.Path, f.LockDir, LockFile, route), true
	}
	return fmt.Sprintf("%s is frozen (%s/%s); %s", f.Path, f.LockDir, LockFile, route), false
}

// repoPath is the root of the repository holding the resolved path and the path relative to it.
func (gd Guard) repoPath(resolved string) (root, rel string, ok bool) {
	root, err := gd.Git.Out(existingDir(resolved), "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", false
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return "", "", false
	}
	rel, err = filepath.Rel(root, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", false
	}
	return root, filepath.ToSlash(rel), true
}

// existingDir is the nearest existing ancestor directory of p, where git can run.
func existingDir(p string) string {
	dir := filepath.Dir(p)
	for dir != filepath.Dir(dir) {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return dir
}

// RealFile resolves symlinks in file, or in its nearest existing parent for a file that does
// not exist yet.
func RealFile(file string) string {
	if r, err := filepath.EvalSymlinks(file); err == nil {
		return r
	}
	dir, rest := filepath.Dir(file), filepath.Base(file)
	for dir != filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		dir, rest = filepath.Dir(dir), filepath.Join(filepath.Base(dir), rest)
	}
	return file
}
