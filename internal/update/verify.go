package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Checksums is the release asset holding one "<sha256>  <asset>" line per archive.
const Checksums = "checksums.txt"

// maxBinary caps how much of an archive Extract decompresses.
const maxBinary = 200 << 20

// Verify checks data against name's line in sums (R-UPD-042): a missing line or another sum is
// an error.
func Verify(sums []byte, name string, data []byte) error {
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || strings.TrimPrefix(f[1], "*") != name {
			continue
		}
		got := sha256.Sum256(data)
		if !strings.EqualFold(f[0], hex.EncodeToString(got[:])) {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
		return nil
	}
	return fmt.Errorf("%s has no entry for %s", Checksums, name)
}

// Extract returns the regular file named ness at the root of a .tar.gz archive, reading at most
// 200 MiB.
func Extract(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("release archive: %w", err)
	}
	tr := tar.NewReader(io.LimitReader(gz, maxBinary))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("archive has no ness binary")
		}
		if err != nil {
			return nil, fmt.Errorf("release archive: %w", err)
		}
		if strings.TrimPrefix(h.Name, "./") != "ness" {
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return nil, errors.New("release archive: ness is not a regular file")
		}
		if h.Size > maxBinary {
			return nil, errors.New("release archive: ness exceeds 200 MiB")
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("release archive: %w", err)
		}
		return data, nil
	}
}
