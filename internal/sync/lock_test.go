package sync_test

import (
	"bytes"
	"strings"
	"testing"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

func TestLockDeterministic(t *testing.T) {
	// covers: R-SYNC-071, R-SYNC-087
	a := nsync.LockEntry{ID: "AGENTS.md#core/router", Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "core", Hash: "h1", Upstream: "h1"}
	b := nsync.LockEntry{ID: ".gitignore#core/ignore", Path: ".gitignore", Kind: nsync.KindBlock, Owner: "core", Hash: "h2", Upstream: "h2"}
	l1 := nsync.Lock{Schema: 1, Ness: "0.1.0", Profiles: map[string]string{"z": "1", "a": "2"}, Entries: []nsync.LockEntry{a, b}}
	l2 := nsync.Lock{Schema: 1, Ness: "0.1.0", Profiles: map[string]string{"a": "2", "z": "1"}, Entries: []nsync.LockEntry{b, a}}
	m1, err1 := l1.Marshal()
	m2, err2 := l2.Marshal()
	if err1 != nil || err2 != nil || !bytes.Equal(m1, m2) {
		t.Fatalf("not deterministic:\n%s\n%s", m1, m2)
	}
	if !strings.Contains(string(m1), `"schema": 1`) || strings.Index(string(m1), ".gitignore") > strings.Index(string(m1), "AGENTS.md") {
		t.Fatalf("lock layout:\n%s", m1)
	}
	back, err := nsync.ParseLock(m1)
	if err != nil || len(back.Entries) != 2 {
		t.Fatalf("ParseLock = %+v %v", back, err)
	}
}

func TestLockSchema(t *testing.T) {
	// covers: R-SYNC-088
	if _, err := nsync.ParseLock([]byte(`{"schema": 2}`)); err == nil || !strings.Contains(err.Error(), "schema 2") {
		t.Fatalf("schema 2 accepted: %v", err)
	}
}

func FuzzParseLock(f *testing.F) {
	// covers: R-SYNC-073
	f.Add([]byte(`{"schema":1,"entries":[{"id":"a","kind":"file"}]}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		l, err := nsync.ParseLock(data)
		if err != nil {
			if !strings.Contains(err.Error(), nsync.LockPath) {
				t.Fatalf("error does not name the lock: %v", err)
			}
			return
		}
		if _, err := l.Marshal(); err != nil {
			t.Fatal(err)
		}
	})
}
