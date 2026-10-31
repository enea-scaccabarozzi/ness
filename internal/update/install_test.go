package update_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestInstallSameDirMode(t *testing.T) {
	// covers: R-UPD-043 — Review focus 3; rejects: temp file in os.TempDir, lost mode, lost .prev
	dir := t.TempDir()
	bin := filepath.Join(dir, "ness")
	writeFile(t, bin, "old")
	writeFile(t, bin+".prev", "older") // rejects: failing or keeping an older .prev
	var probed string
	prev, err := update.Install(bin, []byte("new"), "0.2.0", func(p string) (string, error) {
		probed = p
		return "ness version 0.2.0+abc", nil // rejects: not stripping +build before comparing
	})
	if err != nil || prev != bin+".prev" {
		t.Fatal(prev, err)
	}
	// rejects: probing the installed path after the rename, or a temp file elsewhere
	if filepath.Dir(probed) != dir || probed == bin {
		t.Fatalf("probed %s, want a temp file in %s", probed, dir)
	}
	if b, err := os.ReadFile(bin); err != nil || string(b) != "new" {
		t.Fatalf("installed = %q, %v", b, err)
	}
	if b, err := os.ReadFile(prev); err != nil || string(b) != "old" {
		t.Fatalf("prev = %q, %v", b, err)
	}
	fi, err := os.Stat(bin)
	if err != nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v, %v", fi.Mode(), err)
	}
	if m, err := filepath.Glob(filepath.Join(dir, ".ness-new-*")); err != nil || len(m) != 0 {
		t.Fatal("temp left", m, err)
	}
}

func TestInstallProbeFailsKeepsOld(t *testing.T) {
	// covers: R-UPD-043 — Review focus 3
	for name, probe := range map[string]func(string) (string, error){
		"error": func(string) (string, error) { return "", errors.New("exec format error") }, // rejects: ignoring the probe error
		"wrong": func(string) (string, error) { return "ness version 0.1.9", nil },           // rejects: probe run but output unchecked
	} {
		dir := t.TempDir()
		bin := filepath.Join(dir, "ness")
		writeFile(t, bin, "old")
		if _, err := update.Install(bin, []byte("new"), "0.2.0", probe); err == nil {
			t.Errorf("%s: want error", name)
		}
		if b, err := os.ReadFile(bin); err != nil || string(b) != "old" {
			t.Errorf("%s: old binary = %q, %v", name, b, err)
		}
		// rejects: leaving the temp file, or making .prev before the probe passed
		if m, err := os.ReadDir(dir); err != nil || len(m) != 1 {
			t.Errorf("%s: dir = %v, want only ness", name, m)
		}
	}
}

func TestInstallUnwritableDirKeepsOld(t *testing.T) {
	// covers: R-UPD-043 — rejects: falling back to a temp file outside the binary's directory
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "ness")
	writeFile(t, bin, "old")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	probed := false
	_, err := update.Install(bin, []byte("new"), "0.2.0", func(string) (string, error) { probed = true; return "ness version 0.2.0", nil })
	if err == nil || probed {
		t.Errorf("err = %v, probed = %v; want an error before the probe", err, probed)
	}
	if b, err := os.ReadFile(bin); err != nil || string(b) != "old" {
		t.Errorf("old binary = %q, %v", b, err)
	}
	if m, err := os.ReadDir(dir); err != nil || len(m) != 1 {
		t.Errorf("dir = %v, want only ness", m)
	}
}
