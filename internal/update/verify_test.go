package update_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestVerify(t *testing.T) {
	// covers: R-UPD-042
	data := []byte("archive")
	sum := sha256.Sum256(data)
	// the other asset's line comes first; rejects: checking the first line only
	sums := []byte("0000  ness_0.2.0_linux_arm64.tar.gz\n" + hex.EncodeToString(sum[:]) + "  ness_0.2.0_linux_amd64.tar.gz\n")
	if err := update.Verify(sums, "ness_0.2.0_linux_amd64.tar.gz", data); err != nil {
		t.Fatal(err)
	}
	// rejects: a verify that only checks the asset is listed
	if err := update.Verify(sums, "ness_0.2.0_linux_amd64.tar.gz", []byte("other")); err == nil || !strings.Contains(err.Error(), "checksum mismatch for ness_0.2.0_linux_amd64.tar.gz") {
		t.Fatal(err)
	}
}

func TestVerifyMissingEntry(t *testing.T) {
	// covers: R-UPD-042 — Review focus 3; rejects: "no line" treated as verified
	if err := update.Verify([]byte("abc  other.tar.gz\n"), "ness_0.2.0_linux_amd64.tar.gz", nil); err == nil || !strings.Contains(err.Error(), "no entry for ness_0.2.0_linux_amd64.tar.gz") {
		t.Fatal(err)
	}
	// rejects: substring matching of the name (a longer asset name containing ours)
	sum := sha256.Sum256(nil)
	sums := []byte(hex.EncodeToString(sum[:]) + "  x-ness_0.2.0_linux_amd64.tar.gz\n")
	if err := update.Verify(sums, "ness_0.2.0_linux_amd64.tar.gz", nil); err == nil || !strings.Contains(err.Error(), "no entry for") {
		t.Fatal(err)
	}
}

func TestExtract(t *testing.T) {
	// rejects: taking the first regular file instead of the one named ness
	got, err := update.Extract(tarGz(t, file("README.md", "r"), file("ness", "bin")))
	if err != nil || string(got) != "bin" {
		t.Fatalf("Extract = %q, %v", got, err)
	}
	for name, archive := range map[string][]byte{
		"no ness":  tarGz(t, file("README.md", "r")),     // rejects: empty bytes, nil error
		"nested":   tarGz(t, file("sub/ness", "bin")),    // rejects: matching on base name
		"symlink":  tarGz(t, symlink("ness", "/bin/sh")), // rejects: accepting a non-regular entry
		"not gzip": []byte("plain"),                      // rejects: ignoring the gzip error
	} {
		if _, err := update.Extract(archive); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := update.Extract(tarGz(t, file("README.md", "r"))); err == nil || !strings.Contains(err.Error(), "archive has no ness binary") {
		t.Errorf("no ness: %v", err)
	}
}
