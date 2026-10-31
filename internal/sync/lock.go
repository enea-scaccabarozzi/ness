package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// LockPath is the lock file, relative to the repository root.
const LockPath = ".ness/lock.json"

// Kind is what a lock entry tracks.
type Kind string

// Kinds of target.
const (
	KindFile  Kind = "file"  // a whole file ness owns
	KindSeed  Kind = "seed"  // a file created once, then the user's
	KindBlock Kind = "block" // a managed block inside a shared file
	KindKey   Kind = "key"   // a JSON key ness owns
	KindLink  Kind = "link"  // a symlink
)

// Lock records what ness rendered (R-SYNC-071). Field order is alphabetical so the JSON keys
// come out sorted.
type Lock struct {
	Entries  []LockEntry       `json:"entries"`
	Ness     string            `json:"ness"`
	Profiles map[string]string `json:"profiles"`
	Schema   int               `json:"schema"`
}

// LockEntry is one recorded target. Hash is the SHA-256 of what is on disk as ness wrote it or
// as the user kept it; Upstream is the render it was recorded against. They differ only when
// the user kept their own version (sync --keep).
type LockEntry struct {
	Hash     string `json:"hash"`
	ID       string `json:"id"`
	Kind     Kind   `json:"kind"`
	Owner    string `json:"owner"`
	Path     string `json:"path"`
	Upstream string `json:"upstream"`
}

// ParseLock decodes a lock, refusing a schema this binary does not know (R-SYNC-088).
func ParseLock(data []byte) (*Lock, error) {
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", LockPath, err)
	}
	if l.Schema != 1 {
		return nil, fmt.Errorf("%s: schema %d is not supported by this binary (want 1)", LockPath, l.Schema)
	}
	return &l, nil
}

// readLock reads the repository's lock; a missing lock is an empty one.
func readLock(root *os.Root) (*Lock, error) {
	data, err := root.ReadFile(LockPath)
	if errors.Is(err, fs.ErrNotExist) {
		return &Lock{Schema: 1, Profiles: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseLock(data)
}

// Marshal renders the lock deterministically: entries sorted by id, map keys sorted.
func (l *Lock) Marshal() ([]byte, error) {
	c := *l
	c.Entries = slices.Clone(l.Entries)
	slices.SortFunc(c.Entries, func(a, b LockEntry) int { return strings.Compare(a.ID, b.ID) })
	if c.Entries == nil {
		c.Entries = []LockEntry{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]string{}
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// sum is the lock's content hash: SHA-256, hex (R-SYNC-087).
func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
