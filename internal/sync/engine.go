package sync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
)

// Action is what sync does to one target.
type Action string

// Actions.
const (
	ActNoop     Action = "noop"
	ActCreate   Action = "create"
	ActAdopt    Action = "adopt"
	ActUpdate   Action = "update"
	ActKeep     Action = "keep"     // the user's version stays; upstream is unchanged
	ActConflict Action = "conflict" // held back; the render goes to <path>.ness-new
	ActDelete   Action = "delete"
	ActOrphan   Action = "orphan" // no longer rendered and changed by the user: kept, released
	ActDrop     Action = "drop"   // no longer rendered, nothing to remove: released
)

const (
	absent   = "absent"
	notALink = "\x00not a symlink"
	newExt   = ".ness-new"
)

// Step is the planned action for one target.
type Step struct {
	ID      string
	Action  Action
	want    *Entry
	rec     *LockEntry
	cur     []byte
	present bool
}

// FilePlan is the plan for one path. A conflict in any step holds back the whole file (R-SYNC-045,
// R-SYNC-051); an unreadable or unparsable file is left alone (R-SYNC-046, R-SYNC-055).
type FilePlan struct {
	Path     string
	Steps    []Step
	Before   []byte // current bytes, or the link target for links
	After    []byte // bytes after sync; for a conflict, the render written to <path>.ness-new
	Exists   bool
	Gone     bool
	Link     bool
	Conflict bool
	Kept     bool
	Err      error
}

// LockMeta is what the lock records besides its entries.
type LockMeta struct {
	Ness     string
	Profiles map[string]string
}

// Plan is a full sync plan; nothing is written until Apply.
type Plan struct {
	Files   []FilePlan
	meta    LockMeta
	oldLock []byte
	// Previous is the lock's ness version before this sync; "" without a lock.
	Previous string
}

// Compute plans a sync of entries against the repository and its lock without writing.
func Compute(root *os.Root, entries []Entry, meta LockMeta) (*Plan, error) {
	lock, err := readLock(root)
	if err != nil {
		return nil, err
	}
	oldLock, err := root.ReadFile(LockPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	want := map[string][]Entry{}
	for _, e := range entries {
		e.ID = entryID(e)
		if e.Kind == KindBlock {
			e.Content = normInner(e.Content)
		}
		want[e.Path] = append(want[e.Path], e)
	}
	recs := map[string][]LockEntry{}
	for _, r := range lock.Entries {
		recs[r.Path] = append(recs[r.Path], r)
	}
	paths := slices.Sorted(maps.Keys(want))
	for p := range recs {
		if _, ok := want[p]; !ok {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	plan := &Plan{meta: meta, oldLock: oldLock, Previous: lock.Ness}
	for _, p := range paths {
		plan.Files = append(plan.Files, planFile(root, p, want[p], recs[p]))
	}
	return plan, nil
}

func planFile(root *os.Root, p string, want []Entry, recs []LockEntry) FilePlan {
	fp := FilePlan{Path: p}
	fp.Link = slices.ContainsFunc(want, func(e Entry) bool { return e.Kind == KindLink }) ||
		(len(want) == 0 && slices.ContainsFunc(recs, func(r LockEntry) bool { return r.Kind == KindLink }))
	if err := fp.read(root); err != nil {
		fp.fail(err, recs)
		return fp
	}
	whole := slices.ContainsFunc(want, isWholeEntry) ||
		(len(want) == 0 && slices.ContainsFunc(recs, func(r LockEntry) bool { return isWhole(r.Kind) }))
	if whole {
		fp.planWhole(want, recs)
	} else {
		fp.planParts(want, recs)
	}
	return fp
}

func isWhole(k Kind) bool       { return k == KindFile || k == KindSeed || k == KindLink }
func isWholeEntry(e Entry) bool { return isWhole(e.Kind) }
func (fp *FilePlan) held() bool { return fp.Conflict || fp.Err != nil }
func state(cur []byte, ok bool) string {
	if !ok {
		return absent
	}
	return sum(cur)
}

func (fp *FilePlan) fail(err error, recs []LockEntry) {
	fp.Err = fmt.Errorf("%s: %w", fp.Path, err)
	fp.Steps, fp.Conflict = nil, false // drop what was planned before the failure; each record is kept once below
	for i := range recs {
		fp.Steps = append(fp.Steps, Step{ID: recs[i].ID, Action: ActNoop, rec: &recs[i]})
	}
}

func (fp *FilePlan) read(root *os.Root) error {
	info, err := root.Lstat(fp.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	fp.Exists = true
	isLink := info.Mode()&fs.ModeSymlink != 0
	switch {
	case fp.Link && isLink:
		target, lerr := root.Readlink(fp.Path)
		fp.Before = []byte(target)
		return lerr
	case fp.Link:
		fp.Before = []byte(notALink) // a real file or directory where ness wants a link (R-SYNC-080)
		return nil
	case info.IsDir():
		return errors.New("is a directory")
	case isLink:
		// Writing through the link would change a file ness never recorded (D4).
		// ponytail: only the final path component is checked; a symlinked parent directory inside
		// the repository is read and written through, and os.Root still confines it to the
		// repository (R-SYNC-072). Lstat each parent if a linked directory must be refused too.
		return errors.New("is a symlink; ness manages it as a regular file and leaves it alone")
	}
	fp.Before, err = root.ReadFile(fp.Path)
	return err
}

// decide applies sync rules 2–4 (design §6) to one rendered target.
func decide(cur []byte, present bool, rec *LockEntry, want []byte) Action {
	w, c := sum(want), state(cur, present)
	switch {
	case c == w:
		if rec != nil && rec.Hash == w && rec.Upstream == w {
			return ActNoop
		}
		return ActAdopt
	case rec == nil && !present:
		return ActCreate
	case rec == nil:
		return ActConflict
	}
	return decideRecorded(c, present, rec, w)
}

func decideRecorded(c string, present bool, rec *LockEntry, w string) Action {
	switch {
	case c == rec.Hash && rec.Hash == rec.Upstream:
		return ActUpdate
	case c == rec.Hash && rec.Upstream == w:
		return ActNoop // the user's kept version; upstream unchanged since
	case c == rec.Hash:
		return ActConflict
	case !present:
		return ActCreate
	case rec.Upstream == w:
		return ActKeep
	default:
		return ActConflict
	}
}

func decideSeed(present bool, rec *LockEntry) Action {
	switch {
	case rec != nil:
		return ActNoop
	case present:
		return ActAdopt
	default:
		return ActCreate
	}
}

// orphanAction applies rule 5: only bytes ness wrote and nobody changed are deleted.
func orphanAction(rec *LockEntry, cur []byte, present bool) Action {
	switch {
	case rec.Kind == KindSeed || !present:
		return ActDrop
	case rec.Hash == rec.Upstream && sum(cur) == rec.Hash:
		return ActDelete
	default:
		return ActOrphan
	}
}

func (fp *FilePlan) planWhole(want []Entry, recs []LockEntry) {
	var rec *LockEntry
	for i := range recs {
		if isWhole(recs[i].Kind) && rec == nil {
			rec = &recs[i]
			continue
		}
		fp.Steps = append(fp.Steps, Step{ID: recs[i].ID, Action: ActDrop, rec: &recs[i]})
	}
	fp.After = fp.Before
	if len(want) == 0 {
		a := orphanAction(rec, fp.Before, fp.Exists)
		fp.Steps = append(fp.Steps, Step{ID: rec.ID, Action: a, rec: rec, cur: fp.Before, present: fp.Exists})
		fp.Gone = a == ActDelete
		return
	}
	w := &want[0]
	a := decide(fp.Before, fp.Exists, rec, w.Content)
	if w.Kind == KindSeed {
		a = decideSeed(fp.Exists, rec)
	}
	fp.Steps = append(fp.Steps, Step{ID: w.ID, Action: a, want: w, rec: rec, cur: fp.Before, present: fp.Exists})
	switch a {
	case ActCreate, ActUpdate:
		fp.After = w.Content
	case ActConflict:
		fp.Conflict, fp.After = true, w.Content
	}
}

type partOps struct {
	get    func(data []byte, name string) ([]byte, bool, error)
	set    func(data []byte, name string, v []byte) ([]byte, error)
	remove func(data []byte, name string) ([]byte, error)
}

// opsFor picks the editor by file type; a .toml file gets key ops only for key entries, so a
// profile block in a .toml file keeps working.
func opsFor(p string, kind Kind) partOps {
	if strings.HasSuffix(p, ".json") {
		return partOps{get: jsonGet, set: jsonSet, remove: jsonDelete}
	}
	if strings.HasSuffix(p, ".toml") && kind == KindKey {
		return partOps{get: tomlGet, set: tomlSet, remove: tomlDelete}
	}
	s := styleFor(p)
	return partOps{
		get:    func(d []byte, n string) ([]byte, bool, error) { return getBlock(d, s, n) },
		set:    func(d []byte, n string, v []byte) ([]byte, error) { return setBlock(d, s, n, v) },
		remove: func(d []byte, n string) ([]byte, error) { return removeBlock(d, s, n) },
	}
}

// partName recovers a part's block name or key from its lock id.
func partName(r LockEntry) string {
	if r.Kind == KindBlock {
		return strings.TrimPrefix(r.ID, r.Path+"#")
	}
	return strings.TrimPrefix(r.ID, r.Path+":")
}

// draft is a part file's bytes after sync (data) and its full render for a conflict (forced).
type draft struct {
	ops          partOps
	data, forced []byte
}

func (d *draft) set(name string, v []byte, apply bool) (err error) {
	if d.forced, err = d.ops.set(d.forced, name, v); err != nil || !apply {
		return err
	}
	d.data, err = d.ops.set(d.data, name, v)
	return err
}

func (d *draft) remove(name string) (err error) {
	if d.data, err = d.ops.remove(d.data, name); err != nil {
		return err
	}
	d.forced, err = d.ops.remove(d.forced, name)
	return err
}

func (fp *FilePlan) planParts(want []Entry, recs []LockEntry) {
	var kind Kind
	if len(want) > 0 {
		kind = want[0].Kind
	} else if len(recs) > 0 {
		kind = recs[0].Kind
	}
	d := &draft{ops: opsFor(fp.Path, kind), data: fp.Before, forced: fp.Before}
	if err := fp.planPartSteps(d, want, recs); err != nil {
		fp.fail(err, recs)
		return
	}
	if len(bytes.TrimSpace(fp.Before)) == 0 && strings.HasSuffix(fp.Path, ".json") {
		d.data, d.forced = indentJSON(d.data), indentJSON(d.forced)
	}
	fp.After = d.data
	if fp.Conflict {
		fp.After = d.forced
	}
}

func (fp *FilePlan) planPartSteps(d *draft, want []Entry, recs []LockEntry) error {
	byID := map[string]*LockEntry{}
	for i := range recs {
		byID[recs[i].ID] = &recs[i]
	}
	for i := range want {
		w := &want[i]
		cur, present, err := d.ops.get(fp.Before, w.Name)
		if err != nil {
			return err
		}
		rec := byID[w.ID]
		delete(byID, w.ID)
		a := decide(cur, present, rec, w.Content)
		fp.Steps = append(fp.Steps, Step{ID: w.ID, Action: a, want: w, rec: rec, cur: cur, present: present})
		fp.Conflict = fp.Conflict || a == ActConflict
		if err := d.set(w.Name, w.Content, a == ActCreate || a == ActUpdate); err != nil {
			return err
		}
	}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		if err := fp.planOrphanPart(d, byID[id]); err != nil {
			return err
		}
	}
	return nil
}

func (fp *FilePlan) planOrphanPart(d *draft, rec *LockEntry) error {
	name := partName(*rec)
	cur, present, err := d.ops.get(fp.Before, name)
	if err != nil {
		return err
	}
	a := orphanAction(rec, cur, present)
	fp.Steps = append(fp.Steps, Step{ID: rec.ID, Action: a, rec: rec, cur: cur, present: present})
	if a != ActDelete {
		return nil
	}
	return d.remove(name)
}

// Keep records the user's current version of a conflicted path (sync --keep, R-SYNC-017).
func (p *Plan) Keep(target string) error {
	for i := range p.Files {
		fp := &p.Files[i]
		if fp.Path == target && fp.Conflict {
			fp.Conflict, fp.Kept, fp.After, fp.Gone = false, true, fp.Before, false
			return nil
		}
	}
	return fmt.Errorf("%s is not in conflict; --keep takes a path ness sync reported as a conflict", target)
}

// Conflicts lists the conflicted paths.
func (p *Plan) Conflicts() []string {
	var out []string
	for _, fp := range p.Files {
		if fp.Conflict {
			out = append(out, fp.Path)
		}
	}
	return out
}

// Errors lists the files sync leaves alone because it cannot read or parse them.
func (p *Plan) Errors() []error {
	var out []error
	for _, fp := range p.Files {
		if fp.Err != nil {
			out = append(out, fp.Err)
		}
	}
	return out
}

// Changed reports whether Apply would write any file or the lock (for sync --check).
func (p *Plan) Changed() (bool, error) {
	for _, fp := range p.Files {
		if fp.Writes() {
			return true, nil
		}
	}
	lock, err := p.LockBytes()
	return !bytes.Equal(lock, p.oldLock), err
}

// Writes reports whether Apply changes this path (the .ness-new file of a conflict aside).
func (fp *FilePlan) Writes() bool {
	switch {
	case fp.held():
		return false
	case fp.Gone:
		return true
	case !fp.Exists:
		return fp.After != nil
	default:
		return !bytes.Equal(fp.After, fp.Before)
	}
}

// LockBytes renders the lock Apply writes.
func (p *Plan) LockBytes() ([]byte, error) {
	l := Lock{Schema: 1, Ness: p.meta.Ness, Profiles: p.meta.Profiles}
	for i := range p.Files {
		for _, s := range p.Files[i].Steps {
			if e, ok := p.Files[i].record(s); ok {
				l.Entries = append(l.Entries, e)
			}
		}
	}
	return l.Marshal()
}

func (fp *FilePlan) record(s Step) (LockEntry, bool) {
	switch {
	case fp.Kept && s.want != nil:
		return entryFor(*s.want, state(s.cur, s.present), sum(s.want.Content)), true
	case fp.keepsRecord(s):
		return oldRecord(s)
	case s.want == nil:
		return LockEntry{}, false
	case s.want.Kind == KindSeed && s.Action == ActAdopt:
		return entryFor(*s.want, sum(s.cur), sum(s.want.Content)), true
	default:
		w := sum(s.want.Content)
		return entryFor(*s.want, w, w), true
	}
}

// keepsRecord reports whether a step leaves its target as recorded. A delete held back by keep
// leaves the part in the file, still ness's.
func (fp *FilePlan) keepsRecord(s Step) bool {
	return fp.held() || s.Action == ActNoop || s.Action == ActKeep || (fp.Kept && s.Action == ActDelete)
}

func oldRecord(s Step) (LockEntry, bool) {
	if s.rec == nil {
		return LockEntry{}, false
	}
	return *s.rec, true
}

func entryFor(e Entry, hash, upstream string) LockEntry {
	return LockEntry{ID: e.ID, Path: e.Path, Kind: e.Kind, Owner: e.Owner, Hash: hash, Upstream: upstream}
}

// Apply writes the plan: each file without conflict or error, <path>.ness-new for each
// conflict, and the lock.
func Apply(root *os.Root, p *Plan) error {
	for i := range p.Files {
		if err := p.Files[i].apply(root); err != nil {
			return fmt.Errorf("%s: %w", p.Files[i].Path, err)
		}
	}
	lock, err := p.LockBytes()
	if err != nil || bytes.Equal(lock, p.oldLock) {
		return err
	}
	return writeFile(root, LockPath, lock)
}

func (fp *FilePlan) apply(root *os.Root) error {
	switch {
	case fp.Conflict && !fp.Link: // fail clears Conflict, so a file in error never gets here
		return writeFile(root, fp.Path+newExt, fp.After)
	case fp.held():
		return nil
	}
	if err := root.Remove(fp.Path + newExt); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return fp.write(root)
}

func (fp *FilePlan) write(root *os.Root) error {
	switch {
	case !fp.Writes():
		return nil
	case fp.Gone:
		return root.Remove(fp.Path)
	case fp.Link:
		if fp.Exists {
			if err := root.Remove(fp.Path); err != nil {
				return err
			}
		}
		if err := root.MkdirAll(path.Dir(fp.Path), 0o750); err != nil {
			return err
		}
		return root.Symlink(string(fp.After), fp.Path)
	}
	return writeFile(root, fp.Path, fp.After)
}

func writeFile(root *os.Root, p string, data []byte) error {
	if err := root.MkdirAll(path.Dir(p), 0o750); err != nil {
		return err
	}
	// Replace a symlink rather than write through it to a file ness never recorded (D4).
	if info, err := root.Lstat(p); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		if err := root.Remove(p); err != nil {
			return err
		}
	}
	return root.WriteFile(p, data, 0o644) //nolint:gosec // repository files are world-readable by convention
}
